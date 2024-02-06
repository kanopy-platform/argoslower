package testing

type FakeMeshChecker struct {
	Mesh bool
	Err  error
}

func (m *FakeMeshChecker) OnMesh(ns string) (bool, error) {
	return m.Mesh, m.Err
}
