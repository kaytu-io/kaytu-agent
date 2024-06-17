FROM docker.io/golang:alpine as build
RUN apk --no-cache add wget bash
RUN wget https://raw.githubusercontent.com/kaytu-io/kaytu/main/scripts/install.sh
RUN chmod +x install.sh
RUN ./install.sh


FROM docker.io/golang:alpine
RUN apk --no-cache add ca-certificates bash
COPY ./build/kaytu-agent /
CMD [ "/kaytu-agent" ]