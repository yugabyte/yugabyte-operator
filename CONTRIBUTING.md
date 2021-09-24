## Build instructions:
* [Install operator sdk](https://sdk.operatorframework.io/docs/installation)
* Checkout the source code on your local computer & change into the directory
* If you have already checked out the source code, pull the latest code from github.
* After making your changes, build a docker image of the operator. Make sure you are using `go mod` for dependency management.

```shell
$ export GO111MODULE=on
$ operator-sdk build <your_dockerhub_username>/yb-operator
```

* Run unit tests with below command:

```shell
$ ./hack/runtests.sh
```

* You may push the image to your docker hub account, with below command

```shell
docker push <your_dockerhub_username>/yb-operator
```
