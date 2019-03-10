
from golang:1.12

ARG DIR=$GOPATH/src/github.com/578157900/supermq

RUN mkdir -p $DIR

COPY / $DIR

RUN go install $DIR

CMD ["supermq"]
