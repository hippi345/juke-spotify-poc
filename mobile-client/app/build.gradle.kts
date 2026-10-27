plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

// Override in mobile-client/gradle.properties: JUKE_API_BASE=http://10.0.2.2:9090/
val jukeApiBase: String = run {
    val p = project.findProperty("JUKE_API_BASE") as String?
    // Default 8081: Windows often has PostgreSQL/EnterpriseDB on 8080 (not the Go API). Override via JUKE_API_BASE.
    val raw = p?.trim()?.takeIf { it.isNotEmpty() } ?: "http://10.0.2.2:8081"
    if (raw.endsWith("/")) raw else "$raw/"
}

android {
    namespace = "com.juke.spotifypoc.mobile"
    // splashscreen 1.2.0 requires API 35+. Compose BOM 2026.09.00 and OkHttp 5.5.1
    // require compileSdk 37. targetSdk stays 34 so runtime behavior is unchanged.
    compileSdk = 37

    defaultConfig {
        applicationId = "com.juke.spotifypoc.mobile"
        minSdk = 26
        targetSdk = 34
        versionCode = 2
        versionName = "1.0.1"
        // Must match Go server PORT (default 8081 in server config — avoid Windows:8080 / EDB).
        buildConfigField("String", "API_BASE_URL", "\"${jukeApiBase.replace("\"", "\\\"")}\"")
    }

    buildTypes {
        debug {
        }
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2026.09.00")
    implementation(composeBom)
    androidTestImplementation(composeBom)

    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.core:core-splashscreen:1.2.0")
    implementation("androidx.activity:activity-compose:1.9.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.0")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    debugImplementation("androidx.compose.ui:ui-tooling")

    implementation("com.squareup.retrofit2:retrofit:3.0.0")
    implementation("com.squareup.retrofit2:converter-gson:3.0.0")
    implementation("com.squareup.okhttp3:logging-interceptor:5.5.1")

    implementation("io.coil-kt:coil-compose:2.7.0")

    testImplementation("junit:junit:4.13.2")
}
