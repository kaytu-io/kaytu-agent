FROM docker.io/golang:alpine
RUN apk update
RUN apk --no-cache add ca-certificates bash curl wget
RUN wget https://raw.githubusercontent.com/kaytu-io/kaytu/main/scripts/install.sh
RUN chmod +x install.sh
RUN ./install.sh
RUN rm install.sh
COPY ./build/kaytu-agent /
CMD [ "/kaytu-agent" ]