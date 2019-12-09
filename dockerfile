FROM byuoitav/amd64-alpine
MAINTAINER Daniel Randall <danny_randall@byu.edu>

ARG NAME
ENV name=${NAME}

# add timezone info
RUN apk add tzdata

COPY ${name}-bin ${name}-bin 
COPY version.txt version.txt

# add any required files/folders here
COPY web-dist web-dist

ENTRYPOINT ./${name}-bin
