plugins {
    id("java")
}

group = "io.trigra"
version = "1.0-SNAPSHOT"

repositories {
    mavenCentral()
}

dependencies {
    implementation(libs.bundles.vertx)

    implementation(libs.avaje.inject)
    annotationProcessor(libs.avaje.inject.generator)
    implementation(libs.avaje.http.api)
    implementation("io.avaje:avaje-http-api-vertx:3.8")
    annotationProcessor("io.avaje:avaje-http-vertx-generator:3.8")
    implementation(libs.avaje.jsonb)
    annotationProcessor(libs.avaje.jsonb.generator)

    implementation(libs.gestalt.core)
    implementation(libs.gestalt.yaml.jackson3)
    implementation(libs.gestalt.json.jackson3)

    implementation(libs.bundles.logging)

    implementation(libs.vavr)
    implementation(libs.guava)
    implementation(libs.bundles.commons)
    implementation("io.avaje:avaje-applog-slf4j:1.0")
}

tasks.test {
    useJUnitPlatform()
}