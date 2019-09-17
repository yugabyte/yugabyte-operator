## Build instructions:
* [Install operator sdk](https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md)
* Checkout the source code on your local computer & change into the directory
* If you have already checked out the source code, pull the latest code from github.
* After making your changes, build a docker image of the operator
```shell
operator-sdk build <your_dockerhub_username>/yb-operator
```
* You may push the image to your docker hub account, with below command
```shell
docker push <your_dockerhub_username>/yb-operator
```
