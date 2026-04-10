module trigra.server {
    requires static lombok;
    requires io.avaje.inject;
    requires org.slf4j;
    requires io.vertx.core;
    requires io.avaje.applog;

    provides io.avaje.inject.spi.InjectExtension with io.trigra.factory.FactoryModule;
}