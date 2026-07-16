FROM python:3.12-alpine
RUN addgroup -g 10001 challenge && adduser -D -u 10001 -G challenge challenge
WORKDIR /app
COPY app.py /app/app.py
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=2s --retries=3 CMD wget -q -O - http://127.0.0.1:8080/healthz || exit 1
CMD ["python", "/app/app.py"]
