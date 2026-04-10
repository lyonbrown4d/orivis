package io.trigra;

import io.avaje.inject.BeanScope;
import io.vertx.core.http.HttpServer;
import lombok.Cleanup;
import lombok.extern.slf4j.Slf4j;
import lombok.val;

@Slf4j
public class TrigraServer {

    static void main() {
        @Cleanup val scope = BeanScope.builder().build();
        scope.get(HttpServer.class).requestHandler(request -> {}).connectionHandler(connection -> {}).listen(8080);
    }
}
