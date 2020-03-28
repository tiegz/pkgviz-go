package fake_pkg

type FakeString string
type FakeByte byte
type FakeRune rune

type FakeInt int
type FakeFloat float64
type FakeComplex complex64

type FakeArrayOfStrings []string
type FakeArrayOfArrayOfStrings [][]string

type FakePointerToString *string

type FakeMap map[string]string
type FakeNestedMap map[string]map[string]string

type FakeStruct struct {
	someArrayOfStrings        FakeArrayOfStrings
	someArrayOfArrayOfStrings FakeArrayOfArrayOfStrings
	somePointer               FakePointerToString
	someMap                   FakeMap
	someNestedMap             FakeNestedMap

	FakeString   // implicit field
	PublicField  string
	privateField string
}

type AnotherFakeStruct struct {
	otherTypeStruct       *FakeStruct
	selfReferentialStruct *AnotherFakeStruct
	// TODO fix import
	// nestedStruct          nested.NestedStruct
}
