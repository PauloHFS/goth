package billing

// signatureError é um tipo de erro para falhas de assinatura HMAC
type signatureError struct {
	msg string
}

func (e *signatureError) Error() string {
	return e.msg
}
