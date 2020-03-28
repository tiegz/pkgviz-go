package nested

type nestedStruct struct {
	name                  string
	selfReferentialStruct *NestedStruct
}
