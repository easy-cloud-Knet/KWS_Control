fileName:=kws


build:
	go build -o $(fileName) .

run: build
	chmod +x ./$(fileName)
	./$(fileName)



clean:
	rm ./kws