FROM gcr.io/distroless/static
MAINTAINER Daniel Randall <danny_randall@byu.edu>

COPY atlona-event-forwarder-linux-amd64 /atlona-event-forwarder

ENTRYPOINT ["/atlona-event-forwarder"]
