default:
	@just --list

fmt:
    gofmt -s -w .

push-code: