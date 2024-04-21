package utils

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

func ConvertFieldsStructToByte(src interface{}, dst interface{}) (err error) {
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst).Elem()
	byteArrType := reflect.TypeOf([64]byte{})
	if srcValue.NumField() != dstValue.NumField() {
		return fmt.Errorf("%s", "Разное количество полей у структур")
	}

	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcValue.Field(i)
		dstField := dstValue.Field(i)

		switch srcField.Kind() {
		case reflect.String:
			if dstField.Type() != byteArrType {
				return fmt.Errorf("%s", "Поля получателя должны иметь тип [64]byte")
			}

			srcBytes := []byte(srcField.String())
			if len(srcBytes) > 64 {
				return fmt.Errorf("%s", "Длина строки больше 64 байт")
			}

			copy(dstField.Slice(0, len(srcBytes)).Bytes(), srcBytes)
		case reflect.Struct:
			if dstField.Kind() != reflect.Struct {
				return fmt.Errorf("%s", "Вложенность у структур должна совпадать")
			}

			err = ConvertFieldsStructToByte(srcField.Interface(), dstField.Addr().Interface())
			if err != nil {
				return err
			}
		}
	}

	return
}

func ByteArrToString(arr []byte) string {
	return string(bytes.TrimRightFunc(arr, func(r rune) bool {
		return r == 0
	}))
}

func MagicSNTransform(SN string) string {
	runes := []rune(SN)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func Contains(arr []string, str string) bool {
	for _, val := range arr {
		if strings.Contains(val, str) {
			return true
		}
	}
	return false
}
