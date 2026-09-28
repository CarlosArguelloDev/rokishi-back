package service

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("una-contrasena-segura")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "una-contrasena-segura") {
		t.Fatal("expected password to match")
	}
	if verifyPassword(hash, "contrasena-incorrecta") {
		t.Fatal("unexpected password match")
	}
}

func TestValidatePassword(t *testing.T) {
	if validatePassword("demasiado") == nil {
		t.Fatal("expected short password to fail")
	}
	if validatePassword("doce-caracteres") != nil {
		t.Fatal("expected valid password")
	}
}
