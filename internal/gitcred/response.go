package gitcred

import (
	"fmt"
	"io"
)

type Response struct {
	Username          string
	Password          string
	PasswordExpiryUTC int64
}

func (r Response) Write(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "username=%s\n", r.Username); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "password=%s\n", r.Password); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "password_expiry_utc=%d\n\n", r.PasswordExpiryUTC); err != nil {
		return err
	}
	return nil
}
