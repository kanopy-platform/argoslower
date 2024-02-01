package errors

type Retryable interface {
	IsRetryable() bool
}

type RetryableError struct {
	err       error
	retryable bool
}

func (e *RetryableError) Unwrap() error {
	return e.err
}

func (e *RetryableError) Error() string {
	return e.err.Error()
}

func (e *RetryableError) IsRetryable() bool {
	return e.retryable
}

func NewRetryableError(err error) *RetryableError {
	return &RetryableError{
		err:       err,
		retryable: true,
	}
}

func NewUnretryableError(err error) *RetryableError {
	return &RetryableError{
		err:       err,
		retryable: false,
	}
}
