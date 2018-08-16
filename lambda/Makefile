default:	main.zip

# Need to cross-compile from Mac to Linux!
main:	main.go
	GOOS=linux GOARCH=amd64 go build -o main

main.zip:	main
	zip $@ $^
