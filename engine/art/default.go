package art

import (
	"fmt"
	"strings"
)

// defaultRamp is one of Default's ramps: its name and its colours, dark to
// light, written as hexadecimal in two halves.
type defaultRamp struct {
	name, dark, light string
}

// defaultRamps are Default's 256 colours: sixteen greys from black to white,
// then sixteen ramps of fifteen, the hues in order around the colour wheel
// and then skin, earth and slate. Each ramp steps evenly in lightness, its
// colour strongest in the middle and turning slightly warmer as it lightens.
func defaultRamps() []defaultRamp {
	return []defaultRamp{
		{"grey", "000000 111111 222222 333333 444444 555555 666666 777777",
			"888888 999999 AAAAAA BBBBBB CCCCCC DDDDDD EEEEEE FFFFFF"},
		{"red", "300011 460019 5C0021 740028 8D002E A60034 BD123A D22E2C",
			"E34535 F35D43 FF7554 FF9475 FFB094 FFC9B4 FFE1D3"},
		{"orange", "360605 4C0A04 630F02 781900 8D2600 A23200 B73F00 C15A00",
			"D56900 E57B1D F1903D FBA55A FFBD80 FFD4AC FFEBD6"},
		{"amber", "1B0A02 2E1400 401F00 532B00 663800 794600 8D5400 A06300",
			"B47300 C88300 DC9400 EAA735 F5BD5F FED286 FFE8C0"},
		{"yellow", "160E00 241A00 342700 443500 544300 655200 776100 887100",
			"9A8200 AC9300 BEA400 CFB61F DCC956 E9DC81 F6EEAB"},
		{"lime", "0A1300 142000 202F00 2D3D00 3B4D00 4A5D00 596D00 6A7D00",
			"7B8E00 8D9F00 9FB000 B2C219 C6D252 D9E27D ECF2A8"},
		{"green", "00170E 002518 003522 00452B 005634 00683C 007A43 158E2B",
			"3D9E37 5BAE47 77BD5A 93CC71 AEDB89 C8EAA4 E1F9C1"},
		{"moss", "0C110A 161F10 212D17 2D3C1F 3A4C27 485B32 576B3D 687B4A",
			"798B59 8A9C69 9CAC7B AFBD8F C2CDA4 D5DEBC E9EFD4"},
		{"teal", "011311 00221E 00312B 004039 005147 006256 007465 008674",
			"009983 00AC92 36BDA1 5CCDB1 7EDCC2 A0EBD4 C2F9E6"},
		{"cyan", "011216 002026 002F36 003F47 004F58 005F6A 00717C 00828E",
			"0095A1 00A7B4 10BAC7 4ACBD5 71DBE2 96EAEF BBF8FB"},
		{"sky", "02111C 001F30 002D43 003C56 004B6A 005B7F 006C94 007DA9",
			"008FBF 00A1D5 15B4EA 49C5F6 73D5FF A5E4FF D1F1FF"},
		{"blue", "01072F 031148 071D62 0E297C 173795 2146AD 2D56C3 3B67D7",
			"437AE9 4E8EF7 60A2FF 7BB6FF 98C8FF B6DAFF D6EBFF"},
		{"violet", "110A21 200E40 2E135F 3D197D 4B229B 592EB7 673DD0 744FE8",
			"8261FC 8E7CFF 9D94FF AEABFF C0C0FF D3D5FF E7E9FF"},
		{"magenta", "1C0618 35012C 4B003E 61004F 790061 920072 AC0084 C70095",
			"DD25A4 F041B2 FF5EBE FF88C7 FFA9D2 FFC6DF FFE2EE"},
		{"skin", "2E1817 402220 522D28 643832 76453B 885246 996051 A87158",
			"B88165 C69275 D4A386 E1B599 EDC8AD F9DAC3 FFEFE0"},
		{"earth", "170905 26130C 361F14 472B1D 573726 674531 78533D 856447",
			"947456 A38566 B19678 BFA78B CCB99F DACAB4 E8DCCA"},
		{"slate", "090D17 131927 1E2738 2B3449 37435B 43536D 51627E 5B748E",
			"6B859E 7B96AE 8CA8BE 9FB9CC B3CBDB C7DDE9 DDEEF8"},
	}
}

// DefaultPalette returns Default, the engine's own palette of 256 colours,
// which every project holds and nothing changes. Its IDs are fixed, so it is
// the same in every game and every time it is made.
func DefaultPalette() (*Palette, error) {
	palette := NewPalette()
	for index, entry := range defaultRamps() {
		place := index + 1
		ramp := Ramp{ID: DefaultRampID(place), Name: entry.name, Shades: nil}
		for number, text := range strings.Fields(entry.dark + " " + entry.light) {
			value, err := ParseRGB(text)
			if err != nil {
				return nil, fmt.Errorf("default palette: %s: %w", entry.name, err)
			}
			ramp.Shades = append(ramp.Shades, Shade{
				ID: DefaultShadeID(place, number+1), Color: value, Opacity: MaxOpacity, Label: "",
			})
		}
		if err := palette.InsertRamp(index, ramp); err != nil {
			return nil, fmt.Errorf("default palette: %w", err)
		}
	}

	return palette, nil
}
