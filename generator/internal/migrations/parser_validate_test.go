package migrations

import (
	"testing"
)

func TestValidateGooseMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "correct casing all markers",
			content: "-- +goose Up\nCREATE TABLE t (id int);\n-- +goose Down\nDROP TABLE t;\n",
			wantErr: false,
		},
		{
			name:    "lowercase down",
			content: "-- +goose Up\nCREATE TABLE t (id int);\n-- +goose down\nDROP TABLE t;\n",
			wantErr: true,
		},
		{
			name:    "lowercase up",
			content: "-- +goose up\nCREATE TABLE t (id int);\n-- +goose Down\nDROP TABLE t;\n",
			wantErr: true,
		},
		{
			name:    "lowercase statementbegin",
			content: "-- +goose Up\n-- +goose statementbegin\nSELECT 1;\n-- +goose StatementEnd\n-- +goose Down\nDROP TABLE t;\n",
			wantErr: true,
		},
		{
			name:    "lowercase statementend",
			content: "-- +goose Up\n-- +goose StatementBegin\nSELECT 1;\n-- +goose statementend\n-- +goose Down\nDROP TABLE t;\n",
			wantErr: true,
		},
		{
			name:    "no goose markers",
			content: "CREATE TABLE t (id int);\n",
			wantErr: false,
		},
		{
			name:    "correct casing with statement markers",
			content: "-- +goose Up\n-- +goose StatementBegin\nSELECT 1;\n-- +goose StatementEnd\n-- +goose Down\n-- +goose StatementBegin\nDROP TABLE t;\n-- +goose StatementEnd\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGooseMarkers(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGooseMarkers() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
