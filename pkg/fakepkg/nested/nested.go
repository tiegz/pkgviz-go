package nested

// A struct nested in fakepkg.
type NestedStruct struct {
	name                  string
	selfReferentialStruct *NestedStruct
}
