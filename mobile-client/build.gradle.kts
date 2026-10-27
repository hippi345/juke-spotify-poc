plugins {
    id("com.android.application") version "9.4.1" apply false
    // Compose compiler plugin version must match AGP 9.4's built-in Kotlin (KGP 2.2.10).
    id("org.jetbrains.kotlin.plugin.compose") version "2.2.10" apply false
}
