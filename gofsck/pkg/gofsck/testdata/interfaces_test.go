package model

// This interface should be skipped since it's not a struct
type Repository interface {
	Get(id string) (interface{}, error)
	Save(data interface{}) error
}

// This struct type should also be skipped since it's in a test file
type MockRepository struct{}

func (m *MockRepository) Get(id string) (interface{}, error) {
	return nil, nil
}

func (m *MockRepository) Save(data interface{}) error {
	return nil
}
