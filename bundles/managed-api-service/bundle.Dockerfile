# Execute this docker file via make bundle/build command with VERSION env

FROM scratch

ARG version version

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=managed-api-service
LABEL operators.operatorframework.io.bundle.channels.v1=stable
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable

COPY bundles/managed-api-service/${version}/manifests /manifests/
COPY bundles/managed-api-service/${version}/metadata /metadata/