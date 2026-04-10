package io.trigra.factory;

import io.avaje.inject.Bean;
import io.avaje.inject.Factory;
import io.vertx.core.Vertx;
import io.vertx.core.http.HttpServer;
import org.jspecify.annotations.NonNull;

@Factory
public class VertxFactory {

    @Bean
    Vertx vertx(){
        return Vertx.vertx();
    }

    @Bean
    HttpServer httpServer(@NonNull Vertx vertx){
        return vertx.createHttpServer();
    }
}
