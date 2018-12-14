FROM alpine:3.8

RUN apk update && apk add --no-cache \
    bash \
    curl

ADD ./identityvalidator ./bin/identityvalidator

# Use "kubectl exec <pod name> -- identityvalidator --subscriptionid=<...> --clientid=<...> --resourcegroup=<...>" to execute the validator
ENTRYPOINT [ "/bin/bash", "-c" ]
