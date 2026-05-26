VERSION ?= 0.1.0

.PHONY: build-wheels publish clean

build-wheels:
	pip install go-to-wheel
	go-to-wheel ./cli \
		--name pushkit \
		--version $(VERSION) \
		--set-version-var main.Version \
		--entry-point pushkit \
		--description "PushKit CLI — upload and manage files via an S3-backed API" \
		--url "https://github.com/jayteealao/PushKit" \
		--license MIT \
		--output-dir dist/

publish:
	pip install twine
	twine upload dist/*

clean:
	rm -rf dist/
