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
	someArrayOfStrings        FakeArrayOfStrings
	someArrayOfArrayOfStrings FakeArrayOfArrayOfStrings
	somePointer               FakePointerToString
	someMap                   FakeMap
	someNestedMap             FakeNestedMap

	fakeString   // implicit field
	PublicField  string
	privateField string
}

type anotherFakeStruct struct {
	otherTypeStruct       *FakeStruct
	selfReferentialStruct *AnotherFakeStruct
	// TODO fix import
	// nestedStruct          nested.NestedStruct
}
