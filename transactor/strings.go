package transactor

type StringsSlice []*Strings

func (m StringsSlice) AsSlice() [][]string {
	out := [][]string{}
	for _, strs := range m {
		out = append(out, strs.AsSlice())
	}
	return out
}

func (ss StringsSlice) FromSlice(lsstrs [][]string) []*Strings {
	out := []*Strings{}
	for _, strs := range lsstrs {
		out = append(out, &Strings{Strs: strs})
	}
	return out
}

func (m *Strings) AsSlice() []string {
	return m.Strs
}
