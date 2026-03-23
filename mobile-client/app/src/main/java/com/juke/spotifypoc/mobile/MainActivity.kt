package com.juke.spotifypoc.mobile

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.juke.spotifypoc.mobile.ui.JukeVenueTheme
import com.juke.spotifypoc.mobile.ui.VenueApp

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            JukeVenueTheme {
                Surface(Modifier.fillMaxSize()) {
                    VenueApp()
                }
            }
        }
    }
}
