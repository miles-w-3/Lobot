package command

type CommandBinding[T comparable] struct {
	Key         string
	AltKeys     []string
	Display     string
	Command     T
	Group       *CommandGroup[T]
	Description string
	Searchable  []string // Additional search terms for command palette
}

func (b *CommandBinding[T]) WithAlternates(keys ...string) *CommandBinding[T] {
	b.AltKeys = keys
	return b
}

func (b *CommandBinding[T]) WithDisplay(display string) *CommandBinding[T] {
	b.Display = display
	return b
}

func (b *CommandBinding[T]) WithDescription(desc string) *CommandBinding[T] {
	b.Description = desc
	return b
}

func (b *CommandBinding[T]) WithSearchTerms(terms ...string) *CommandBinding[T] {
	b.Searchable = terms
	return b
}
