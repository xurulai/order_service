package errno

import "errors"

var (
	ErrQueryFailed = errors.New("query db failed")

	ErrUnderstock = errors.New("understock")

	ErrUpdateFailed = errors.New("update data failed")

	ErrOrderNotFound = errors.New("not found order")
)
