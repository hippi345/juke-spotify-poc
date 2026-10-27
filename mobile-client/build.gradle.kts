plugins {
    id("com.android.application") version "8.13.2" apply false
    // Retrofit 3.0.0 and its stdlib are Kotlin 2.1 metadata; 1.9.23 cannot read it.
    id("org.jetbrains.kotlin.android") version "2.1.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.1.21" apply false
}
