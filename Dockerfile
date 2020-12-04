FROM golang:buster AS build-env
WORKDIR /app
ADD . /app
RUN apt-get update && apt-get upgrade -y
RUN cd /app && make requirements && make -B pastad

FROM debian:buster
RUN apt-get update && apt-get upgrade -y
RUN mkdir /app
RUN mkdir /data
WORKDIR /data
COPY --from=build-env /app/pastad /app/pastad
ENTRYPOINT /app/pastad
VOLUME ["/data"]
