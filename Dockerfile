FROM registry.ci.openshift.org/openshift/release:golang-1.15 AS builder
WORKDIR /go/src/github.com/openshift/cluster-api-provider-ibmcloud
COPY . .
# VERSION env gets set in the openshift/release image and refers to the golang version, which interfers with our own
RUN unset VERSION \
 && GOPROXY=off NO_DOCKER=1 make build

FROM registry.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/cluster-api-provider-ibmcloud/bin/machine-controller-manager /
COPY --from=builder /go/src/github.com/openshift/cluster-api-provider-ibmcloud/bin/termination-handler /
