package framework

import (
	"reflect"
	"testing"
)

type fakeData struct {
	data string
}

func (f *fakeData) Clone() StateData {
	copy := &fakeData{
		data: f.data,
	}
	return copy
}

var fakeKey StateKey = "fakedata_key"

func TestCycleState_WriteAndRead(t *testing.T) {
	tests := []struct {
		name    string
		state   *CycleState
		key     StateKey
		value   StateData
		want    StateData
		wantErr bool
	}{
		{
			name:  "test Write and Read",
			state: NewCycleState(),
			key:   fakeKey,
			value: &fakeData{data: "modified"},
			want:  &fakeData{data: "modified"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.state.Write(tt.key, tt.value)
			data, err := tt.state.Read(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			s, ok := data.(*fakeData)
			if !ok {
				t.Errorf("invalid fakeData state, got type %T", data)
				return
			}
			if !reflect.DeepEqual(s, tt.want) {
				t.Errorf("Read() got = %v, want %v", s, tt.want)
			}
		})
	}
}
