package fakepkg

type fakeString string
type fakeByte byte
type fakeRune rune

type fakeInt int
type fakeFloat float64
type fakeComplex complex64

type fakeArrayOfStrings []string
type fakeArrayOfArrayOfStrings [][]string

type fakePointerToString *string

type fakeMap map[string]string
type fakeNestedMap map[string]map[string]string

type fakeStruct struct {
	someArrayOfStrings        fakeArrayOfStrings
	someArrayOfArrayOfStrings fakeArrayOfArrayOfStrings
	somePointer               fakePointerToString
	someMap                   fakeMap
	someNestedMap             fakeNestedMap

	fakeString   // implicit field
	PublicField  string
	privateField string
}

type anotherFakeStruct struct {
	otherTypeStruct       *fakeStruct
	selfReferentialStruct *anotherFakeStruct
	// TODO fix import
	// nestedStruct          nested.NestedStruct
}
