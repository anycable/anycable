FROM ruby:2.6.0-alpine

RUN apk add --no-cache --update build-base \
                                linux-headers \
                                postgresql-dev \
                                tzdata \
                                git \
                                nodejs \
                                yarn \
                                libc6-compat

WORKDIR /home/app
COPY Gemfile Gemfile.lock /home/app/
RUN BUNDLE_FORCE_RUBY_PLATFORM=1 bundle install --without development test --retry 5

COPY . /home/app/
EXPOSE 50051

ENTRYPOINT ["bundle", "exec"]
CMD ["anycable"]
