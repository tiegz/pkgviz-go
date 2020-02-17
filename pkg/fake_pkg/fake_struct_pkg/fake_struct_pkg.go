package fake_struct_pkg

type FakeStructA struct {
	privateName string
	publicName  string
}

type FakeStructB struct {
	otherTypeStruct *FakeStructA
	sameTypeStruct  *FakeStructB
}
