package sbac // import "chainspace.io/prototype/sbac"

// StringsSlice ...
type StringsSlice []*Strings

// AsSlice returns a map of Strings as a slice
func (s *Strings) AsSlice() []string {
	return s.Strs
}

// AsSlice returns a map of StringsSlice as a slice
func (ss StringsSlice) AsSlice() [][]string {
	out := [][]string{}
	for _, strs := range ss {
		out = append(out, strs.AsSlice())
	}
	return out
}

// FromSlice returns a map of Strings from a StringsSlice
func (ss StringsSlice) FromSlice(mapOfStrs [][]string) []*Strings {
	out := []*Strings{}
	for _, strs := range mapOfStrs {
		out = append(out, &Strings{Strs: strs})
	}
	return out
}
