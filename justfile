default:
	@just --list

fmt:
    gofmt -s -w .

dl-model model:
    mkdir -p ./models/{{model}}
    huggingface-cli download {{model}} --local-dir ./models/{{model}}
