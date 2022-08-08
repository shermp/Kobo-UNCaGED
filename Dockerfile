FROM aricodes/kobo-toolchain:latest

WORKDIR /uncaged
COPY . .

ENTRYPOINT ["./docker-entrypoint.sh"]
