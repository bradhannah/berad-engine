package window

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"runtime/debug"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// openedQueue is how many windows can finish connecting between two frames.
const openedQueue = 16

// errPaneCrashed reports a pane that panicked.
var errPaneCrashed = errors.New("pane crashed")

// shell runs the editor's windows as panes of one window (increment 10). The
// project browser's process shows every window the orchestrator opens after
// it as a tab of its own. Each pane keeps its own connection, so to the
// orchestrator it is a window as before; the shell gives each its part of the
// window, its input and its tab.
type shell struct {
	args     Args
	registry *plugin.Registry
	theme    *ui.Theme
	reader   *inputReader
	tabs     []*tab
	active   int
	size     image.Point
	opened   chan openedPane
	title    string
	// shots are screenshots the panes asked for, of the whole window.
	shots []string
	// clock is the pinned clock, clockOpen whether its panel shows, and
	// clockLive whether a preview ran when it was last read.
	clock     *pane
	clockOpen bool
	clockLive bool
	// dragging is set while the divider of a split tab is dragged.
	dragging bool
	// stats are the window's frame statistics, when they are asked for.
	stats *frameStats
	// lastTab is each kind's tab last in front.
	lastTab map[string]*tab
}

// tab is one tab of the window bar: one pane, or two side by side, the left
// taking split of its width.
type tab struct {
	panes []*pane
	focus int
	split float64
}

// pane is one window shown in the shell: its host, its part of the shell's
// window and the image it draws into. failed is set when it crashed while
// drawing, and it closes on the next tick.
type pane struct {
	host   *host
	view   string
	target ids.ULID
	place  *paneSurface
	image  *ebiten.Image
	rect   image.Rectangle
	failed error
}

// openedPane is a window connected for the shell, or why it could not be.
type openedPane struct {
	args Args
	conn *client.Client
	err  error
}

// runShell runs the project browser's window as a shell until its last pane
// closes. It must be called from the main goroutine, as Ebitengine requires.
func runShell(args Args, registry *plugin.Registry) error {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := connect(ctx, args, registry)
	if err != nil {
		return err
	}
	theme, err := ui.NewTheme()
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	defer theme.Close()
	shown := newShell(args, registry, theme)
	project, err := shown.addPane(conn, args)
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	project.host.closeQuits = true
	project.host.hosts = shown.host
	project.host.popOut = nil
	// The pinned clock is a Clock window, always running.
	project.host.Open(clockview.ViewName, args.Target, false)
	ebiten.SetWindowTitle(project.host.view.Title(conn))
	ebiten.SetWindowSize(windowWidth, windowHeight+rowHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Closing the shell's window quits the editor, asking first.
	ebiten.SetWindowClosingHandled(true)
	var options ebiten.RunGameOptions
	options.InitUnfocused = scripted()
	runErr := ebiten.RunGameWithOptions(timed(shown, shown.stats), &options)
	if errors.Is(runErr, ebiten.Termination) {
		runErr = nil
	}

	return errors.Join(runErr, shown.closeAll(), shown.stats.write())
}

func newShell(args Args, registry *plugin.Registry, theme *ui.Theme) *shell {
	return &shell{
		args: args, registry: registry, theme: theme, reader: newInputReader(), tabs: nil, active: 0,
		size: image.Pt(0, 0), opened: make(chan openedPane, openedQueue), title: "", shots: nil,
		clock: nil, clockOpen: false, clockLive: false, dragging: false, stats: newFrameStats("shell"),
		lastTab: map[string]*tab{},
	}
}

// addPane shows a connected window in a new tab, in front.
func (s *shell) addPane(conn *client.Client, args Args) (*pane, error) {
	shown, err := s.newPane(conn, args)
	if err != nil {
		return nil, err
	}
	shown.host.popOut = func() { popOut(shown) }
	s.place(shown)

	return shown, nil
}

// popOut moves a pane into a window process of its own. The orchestrator
// starts the window, then tells the pane to close, saving first, once the
// window has joined, so the undo list they share carries over.
func popOut(shown *pane) {
	shown.host.frame.Status("Opening this in a window of its own")
	shown.host.background(func(ctx context.Context) func() {
		result, err := shown.host.conn.PopOut(ctx, shown.view, shown.target, shown.host.conn.Window())
		if err == nil {
			return nil
		}

		return func() { shown.host.frame.Report("")(result, err) }
	})
}

// newPane runs a connected window's view in a pane.
func (s *shell) newPane(conn *client.Client, args Args) (*pane, error) {
	script, err := loadScript(args.View)
	if err != nil {
		return nil, err
	}
	place := newPaneSurface()
	shown := &pane{
		host: newHost(conn, newView(args, conn.Window(), s.theme), args.View, s.theme, place),
		view: args.View, target: args.Target, place: place, image: nil, rect: ui.Zero(), failed: nil,
	}
	shown.host.script = script
	shown.host.saveShot = s.queueShot

	return shown, nil
}

// host connects a window the orchestrator asked the shell to show, off the
// frame loop; its pane appears once it has connected.
func (s *shell) host(asked protocol.Host) {
	args := Args{
		Socket: s.args.Socket, View: asked.View, Target: asked.Target, Window: asked.Window, Build: s.args.Build,
		Shell: false,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		conn, err := connect(ctx, args, s.registry)
		s.opened <- openedPane{args: args, conn: conn, err: err}
	}()
}

// receiveOpened shows the windows that finished connecting.
func (s *shell) receiveOpened() {
	for {
		select {
		case opened := <-s.opened:
			s.show(opened)
		default:
			return
		}
	}
}

func (s *shell) show(opened openedPane) {
	if opened.err == nil {
		var err error
		if opened.args.View == clockview.ViewName && s.clock == nil {
			err = s.pinClock(opened.conn, opened.args)
		} else {
			_, err = s.addPane(opened.conn, opened.args)
		}
		if err == nil {
			return
		}
		opened.err = errors.Join(err, opened.conn.Close())
	}
	if panes := s.panes(); len(panes) > 0 {
		panes[0].host.frame.Status("Could not open a " + opened.args.View + " window: " + opened.err.Error())
	}
}

// Update runs every pane once per tick. The focused pane gets the frame's
// input and the others none, so hidden tabs keep working.
func (s *shell) Update() error {
	s.receiveOpened()
	s.layout()
	input := s.readInput(s.target(s.focused()))
	s.updateBar(&input)
	s.updateSplit(&input)
	s.layout()
	focused := s.focused()
	s.route(input, s.target(focused), focused)
	for _, shown := range s.panes() {
		s.run(shown)
	}
	s.attend()
	s.remember()
	s.followClock()
	s.retitle()
	if len(s.tabs) == 0 {
		return ebiten.Termination
	}

	return nil
}

// layout gives every pane its part of the window. A pane of a hidden tab
// keeps the size it will be shown at.
func (s *shell) layout() {
	for _, each := range s.tabs {
		layoutTab(each, s.area())
	}
	if s.clock != nil {
		s.clock.rect = s.panelRect()
	}
}

// target is the pane that takes the frame's input: the clock's panel while
// the cursor is over it or its time field is being typed in, and otherwise
// the focused pane of the tab in front.
func (s *shell) target(focused *pane) *pane {
	if !s.clockOpen || s.clock == nil {
		return focused
	}
	if image.Pt(ebiten.CursorPosition()).In(s.clock.rect) || s.clock.host.view.Focused() != nil {
		return s.clock
	}

	return focused
}

// readInput reads the frame's input for the focused pane's text field.
func (s *shell) readInput(focused *pane) ui.Input {
	var field *ui.TextField
	if focused != nil {
		field = focused.host.view.Focused()
		s.reader.offset = focused.rect.Min
	}

	return s.reader.read(field)
}

// route gives the target pane the frame's input, in its own coordinates, and
// every other pane none. Closing the shell's window asks the focused pane to
// quit the editor.
func (s *shell) route(input ui.Input, target, focused *pane) {
	for _, shown := range s.panes() {
		shown.place.input = ui.NewInput(outside())
	}
	if focused != nil && ebiten.IsWindowBeingClosed() {
		focused.place.closing = closeQuit
	}
	if target == nil {
		return
	}
	input.Cursor = input.Cursor.Sub(target.rect.Min)
	target.place.input = input
}

// run runs one pane's tick. A pane that ends or crashes closes alone.
func (s *shell) run(shown *pane) {
	err := shown.failed
	if err == nil {
		shown.host.Layout(shown.rect.Dx(), shown.rect.Dy())
		err = safely(shown.host.Update)
	}
	// A pane's screenshot is of the whole window, even while its tab is
	// hidden.
	if shown.host.shot != "" {
		s.shots = append(s.shots, shown.host.shot)
		shown.host.shot = ""
	}
	if err != nil {
		s.close(shown, err)
	}
}

// safely runs part of a pane's tick, turning a panic into an error.
func safely(run func() error) error {
	var failure error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				failure = fmt.Errorf("%w: %v\n%s", errPaneCrashed, recovered, debug.Stack())
			}
		}()
		failure = run()
	}()

	return failure
}

// close takes away a pane that ended: closed by its author or by the
// orchestrator, or crashed, which the other panes are told of.
func (s *shell) close(shown *pane, cause error) {
	if !errors.Is(cause, ebiten.Termination) {
		logLine("a " + shown.view + " pane crashed: " + cause.Error())
		for _, other := range s.panes() {
			if other != shown {
				other.host.frame.Notice("A " + shown.view + " tab closed unexpectedly. Its failure is in the editor's log.")
			}
		}
	}
	s.stats.addLatencies(shown.host.frame.Latencies())
	if err := errors.Join(shown.host.sounds.Close(), shown.host.conn.Close()); err != nil {
		logLine("close a " + shown.view + " pane: " + err.Error())
	}
	if shown.image != nil {
		shown.image.Deallocate()
	}
	s.remove(shown)
}

// remove takes a pane out of its tab, and an empty tab off the bar.
func (s *shell) remove(shown *pane) {
	if shown == s.clock {
		s.clock, s.clockOpen = nil, false

		return
	}
	for index, each := range s.tabs {
		for slot, candidate := range each.panes {
			if candidate != shown {
				continue
			}
			each.panes = append(each.panes[:slot], each.panes[slot+1:]...)
			each.focus = min(each.focus, max(len(each.panes)-1, 0))
			if len(each.panes) == 0 {
				s.tabs = append(s.tabs[:index], s.tabs[index+1:]...)
				if s.active >= index && s.active > 0 {
					s.active--
				}
			}

			return
		}
	}
}

// attend brings forward a pane that asked for attention: its tab is shown and
// it is focused, or the clock's panel opens, and the window asks for
// attention when it is not in front.
func (s *shell) attend() {
	if s.clock != nil && s.clock.place.attention {
		s.clock.place.attention, s.clockOpen = false, true
	}
	for index, each := range s.tabs {
		for slot, shown := range each.panes {
			if !shown.place.attention {
				continue
			}
			shown.place.attention = false
			s.active, each.focus = index, slot
			if !ebiten.IsFocused() {
				ebiten.RequestAttention()
			}
		}
	}
}

// retitle names the window after its focused pane.
func (s *shell) retitle() {
	focused := s.focused()
	if focused == nil || focused.place.title == s.title {
		return
	}
	s.title = focused.place.title
	ebiten.SetWindowTitle(s.title)
}

// panes lists every pane, shown or not, the pinned clock's last.
func (s *shell) panes() []*pane {
	var all []*pane
	for _, each := range s.tabs {
		all = append(all, each.panes...)
	}
	if s.clock != nil {
		all = append(all, s.clock)
	}

	return all
}

// area is the part of the window under the bar, where tabs show.
func (s *shell) area() image.Rectangle {
	return image.Rect(0, s.barHeight(), s.size.X, s.size.Y)
}

// front is the tab in front, if there is one.
func (s *shell) front() *tab {
	if s.active >= len(s.tabs) {
		return nil
	}

	return s.tabs[s.active]
}

// focused is the pane that takes the keyboard: the focused pane of the tab in
// front.
func (s *shell) focused() *pane {
	front := s.front()
	if front == nil {
		return nil
	}

	return front.panes[front.focus]
}

// Draw draws the tab in front and the window bar.
func (s *shell) Draw(screen *ebiten.Image) {
	screen.Fill(ui.ColorBackground())
	if front := s.front(); front != nil {
		for _, shown := range front.panes {
			drawPane(screen, shown)
		}
		s.drawSplit(screen, front)
	}
	s.drawClock(screen)
	s.drawBar(screen)
	for _, path := range s.shots {
		if err := saveScreenshot(screen, path); err != nil {
			logLine(err.Error())
		}
	}
	s.shots = nil
}

// queueShot saves a pane's screenshot as one of the whole window, once the
// window is drawn.
func (s *shell) queueShot(_ *ebiten.Image, path string) error {
	s.shots = append(s.shots, path)

	return nil
}

// drawPane draws a pane into its own image, so its view draws from (0, 0),
// and that image into its part of the window.
func drawPane(screen *ebiten.Image, shown *pane) {
	size := shown.rect.Size()
	if size.X <= 0 || size.Y <= 0 || shown.failed != nil {
		return
	}
	if shown.image == nil || shown.image.Bounds().Size() != size {
		if shown.image != nil {
			shown.image.Deallocate()
		}
		shown.image = ebiten.NewImage(size.X, size.Y)
	}
	shown.image.Clear()
	shown.failed = safely(func() error {
		shown.host.Draw(shown.image)

		return nil
	})
	var options ebiten.DrawImageOptions
	options.GeoM.Translate(float64(shown.rect.Min.X), float64(shown.rect.Min.Y))
	screen.DrawImage(shown.image, &options)
}

// Layout uses the window's real size, one screen pixel per pixel.
func (s *shell) Layout(outsideWidth, outsideHeight int) (int, int) {
	s.size = image.Pt(outsideWidth, outsideHeight)

	return outsideWidth, outsideHeight
}

// closeAll closes the panes still open when the window closes.
func (s *shell) closeAll() error {
	var err error
	for _, shown := range s.panes() {
		s.stats.addLatencies(shown.host.frame.Latencies())
		err = errors.Join(err, shown.host.sounds.Close(), shown.host.conn.Close())
	}

	return err
}

// logLine writes a line to the shell's log, its standard error. If that
// fails there is nowhere left to report to.
func logLine(message string) {
	if _, err := fmt.Fprintln(os.Stderr, "shell: "+message); err != nil {
		return
	}
}
