package ui

// PaintsOf is each stretch of a text as the field draws it: its text and
// its paint, in order.
func (w *TextField) PaintsOf(value string) [][2]string {
	var painted [][2]string
	for _, span := range w.spans(value) {
		painted = append(painted, [2]string{span.Text, span.Paint})
	}

	return painted
}
