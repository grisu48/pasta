FROM registry.suse.com/bci/golang AS build-env
WORKDIR /app
ADD . /app
RUN cd /app && make requirements && make pastad-static

FROM scratch
WORKDIR /data
COPY --from=build-env /app/pastad /app/mime.types /app/
ENTRYPOINT ["/app/pastad", "-m", "/app/mime.types", "-c", "/data/pastad.toml"]
VOLUME ["/data"]
