package imgio

// OCI implements writing to OCI format image trees. These trees are then
// tarred and compressed for distribution purposes.
type OCI struct{}

// NewOCI creates a new *OCI.
func NewOCI() *OCI {
	return &OCI{}
}
