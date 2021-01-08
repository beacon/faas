package sets

// Empty define a dummy struct with minimal memory usage
type Empty struct{}

// String set
type String map[string]Empty

// Add entry
func (s String) Add(v string) {
	s[v] = Empty{}
}
