FROM nginx:1.27-alpine

RUN rm /etc/nginx/conf.d/default.conf && \
    mkdir -p /var/www/certbot /etc/nginx/certs

COPY nginx.conf /etc/nginx/nginx.conf
COPY certs/ /etc/nginx/certs/

EXPOSE 80 443
