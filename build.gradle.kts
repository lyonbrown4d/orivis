import name.remal.gradle_plugins.lombok.LombokPlugin

plugins {
    `java-library`
    alias(libs.plugins.lombok) apply false
}

group = "io.trigra"
version = "1.0-SNAPSHOT"

allprojects {
    repositories {
        mavenLocal()
        mavenCentral()
        google()
        gradlePluginPortal()
    }
}

subprojects {
    apply<JavaLibraryPlugin>()
    apply<LombokPlugin>()

    dependencies {
        compileOnly(rootProject.libs.record.builder.core)
        annotationProcessor(rootProject.libs.record.builder.processor)
        implementation(rootProject.libs.mapstruct)
        annotationProcessor(rootProject.libs.mapstruct.processor)
    }

    tasks.test {
        useJUnitPlatform()
    }
}
