make docker-build
docker tag controller:latest harbor.seafood-dev.com/test/controller:latest
docker push harbor.seafood-dev.com/test/controller:latest
kubectl delete ns crd-system
make install
make deploy