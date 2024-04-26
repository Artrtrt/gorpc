package utils

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

func StructFieldsToByte(src interface{}, dst interface{}) (err error) {
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst).Elem()
	srcType := srcValue.Type()
	dstType := dstValue.Type()
	byteArrType := reflect.TypeOf([64]byte{})
	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcValue.Field(i)
		srcFieldName := srcType.Field(i).Name
		dstTypeField, ok := dstType.FieldByName(srcFieldName)
		if !ok {
			return fmt.Errorf("%s %s", "Не найдено поле у получателя", srcFieldName)
		}

		dstField := dstValue.FieldByName(srcFieldName)
		switch srcField.Kind() {
		case reflect.String:
			if dstTypeField.Type != byteArrType {
				return fmt.Errorf("%s", "Поля получателя должны иметь тип [64]byte")
			}

			srcBytes := []byte(srcField.String())
			if len(srcBytes) > 64 {
				return fmt.Errorf("%s", "Длина строки больше 64 байт")
			}

			copy(dstField.Slice(0, len(srcBytes)).Bytes(), srcBytes)
		case reflect.Struct:
			if dstTypeField.Type.Kind() != reflect.Struct {
				return fmt.Errorf("%s", "Вложенность у структур должна совпадать")
			}

			err = StructFieldsToByte(srcField.Interface(), dstField.Addr().Interface())
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s %s", "Неподдерживаемый тип поля", srcFieldName)
		}
	}

	return
}

func StructFieldsToString(src interface{}, dst interface{}) (err error) {
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst).Elem()
	srcType := srcValue.Type()
	dstType := dstValue.Type()
	byteArrType := reflect.TypeOf([64]byte{})

	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcValue.Field(i)
		srcFieldName := srcType.Field(i).Name
		dstField, ok := dstType.FieldByName(srcFieldName)
		if !ok {
			return fmt.Errorf("%s %s", "Не найдено поле у получателя", srcFieldName)
		}

		switch srcField.Kind() {
		case reflect.Array:
			if srcField.Type() != byteArrType {
				return fmt.Errorf("%s %s", "Поля исходной структуры должны иметь тип [64]byte для поля", srcFieldName)
			}

			srcBytes := make([]byte, srcField.Len())
			for i := 0; i < srcField.Len(); i++ {
				srcBytes[i] = srcField.Index(i).Interface().(byte)
			}

			dstValue.FieldByName(srcFieldName).SetString(ByteArrToString(srcBytes))
		case reflect.Struct:
			if dstField.Type.Kind() != reflect.Struct {
				return fmt.Errorf("%s %s", "Вложенность у структур должна совпадать для поля", srcFieldName)
			}

			err = StructFieldsToString(srcField.Interface(), dstValue.FieldByName(srcFieldName).Addr().Interface())
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s %s", "Неподдерживаемый тип поля", srcFieldName)
		}
	}

	return
}

func ByteArrToString(arr []byte) string {
	return string(bytes.TrimRightFunc(arr, func(r rune) bool {
		return r == 0
	}))
}

// func GenerateUUID(serialNumber string) uuid.UUID {
// 	uniqueInfo := serialNumber + time.Now().Format("20060102150405.000")
// 	uniqueBytes := []byte(uniqueInfo)

// 	hasher := md5.New()
// 	hasher.Write(uniqueBytes)
// 	hash := hasher.Sum(nil)
// 	uuid.NewMD5(uuid.UUID{}, hash)
// 	return uuid.NewMD5(uuid.UUID{}, hash)
// }

func Contains(arr []string, str string) bool {
	for _, val := range arr {
		if strings.Contains(val, str) {
			return true
		}
	}
	return false
}
