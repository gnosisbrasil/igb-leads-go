package service

import "fmt"

// AppError mirrors utils/errors.js: a client error with an HTTP status.
type AppError struct {
	Status  int
	Message string
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%d: %s", e.Status, e.Message)
}

// AsAppError unwraps err to an *AppError when it is one.
func AsAppError(err error) (*AppError, bool) {
	if ae, ok := err.(*AppError); ok {
		return ae, true
	}
	return nil, false
}

func NotFound(message string) *AppError {
	if message == "" {
		message = "Recurso não encontrado"
	}
	return &AppError{Status: 404, Message: message}
}

func Forbidden(message string) *AppError {
	if message == "" {
		message = "Acesso negado"
	}
	return &AppError{Status: 403, Message: message}
}

func BadRequest(message string) *AppError {
	return &AppError{Status: 400, Message: message}
}

func Unauthorized(message string) *AppError {
	if message == "" {
		message = "Não autenticado"
	}
	return &AppError{Status: 401, Message: message}
}
