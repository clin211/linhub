package otelslog // import "go.opentelemetry.io/contrib/bridges/otelslog"

// 生成 convert 相关文件:
//go:generate gotmpl --body=../../internal/shared/logutil/convert_test.go.tmpl "--data={ \"pkg\": \"otelslog\" }" --out=convert_test.go
//go:generate gotmpl --body=../../internal/shared/logutil/convert.go.tmpl "--data={ \"pkg\": \"otelslog\" }" --out=convert.go
