package rest

const (
	Success             = "200"
	ServerInternalError = "500"
)

var ErrorCode = map[string]string{
	Success:             "Success",
	ServerInternalError: "Server Internal Error",
}

func Error(error error) (string, string) {
	err := ErrorCode[error.Error()]
	if err == "" {
		return ServerInternalError, ErrorCode[ServerInternalError]
	}
	return error.Error(), err
}
