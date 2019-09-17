PACKAGES=$(go list ./... | grep '/pkg/')
for package in $PACKAGES; do
    go test -v ${@} "$package" -coverprofile=coverage.out
done
