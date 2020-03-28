package nested

type NestedStruct struct {
	name                  string
	selfReferentialStruct *NestedStruct
}
