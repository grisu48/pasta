ARG ARCH=

FROM ${ARCH}golang:buster AS build-env
WORKDIR /app
ADD . /app
#RUN apt-get update && apt-get upgrade -y
RUN cd /app && make requirements && make pastad-static

FROM scratch
WORKDIR /data
COPY --from=build-env /app/pastad /app/mime.types /app/
ENTRYPOINT ["/app/pastad", "-m", "/app/mime.types", "-c", "/data/pastad.toml"]
VOLUME ["/data"]
