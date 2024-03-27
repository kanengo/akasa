package codegen

import "testing"

func TestGenerate(t *testing.T) {
	err := Generate("E:\\Codes\\github\\akasar\\internal\\codegen\\test", nil)
	if err != nil {
		return
	}
}
