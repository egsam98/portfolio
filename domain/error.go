package domain

// Error defines client-side errors
type Error string

func (e Error) Error() string {
	return string(e)
}
