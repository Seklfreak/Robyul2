FROM sn0w/go-ci

EXPOSE 1337

VOLUME /karen
WORKDIR /karen

ENTRYPOINT /karen/karen
