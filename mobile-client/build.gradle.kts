plugins {
    id("com.android.application") version "9.4.1" apply false
    // Compose compiler plugin version must match AGP 9.4's built-in Kotlin (KGP 2.2.10).
    // KGP 2.2.10 can read Retrofit 3.0.0's Kotlin metadata 2.1.
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.21" apply false
}
