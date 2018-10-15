package auth

import (
	"testing"
)

func TestGenerateToken(t *testing.T) {
	t.Run("Success No Data", func(t *testing.T) {
		blank := make(map[string]interface{})
		token, err := GenerateToken(blank)

		if err != nil {
			t.Fatal("Error generating token:", err)
		}

		if token.Token == "" {
			t.Fatal("Token is empty")
		}

		err = ValidateToken(token.Token, &blank)

		if err != nil {
			t.Fatal("Invalid token:", err)
		}
	})

	t.Run("Success With Data", func(t *testing.T) {
		d1 := make(map[string]string)
		d1["foo"] = "bar"
		token, err := GenerateToken(d1)

		if err != nil {
			t.Fatal("Error generating token:", err)
		}

		d2 := make(map[string]string)
		err = ValidateToken(token.Token, &d2)

		if err != nil {
			t.Fatal("Invalid token:", err)
		}

		if d1["foo"] != d2["foo"] {
			t.Fatal("data doesn't match:", d1, d2)
		}
	})
}
