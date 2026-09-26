package com.juke.spotifypoc.mobile.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class FormatTimeTest {

    @Test
    fun formatMs_zeroOrNegative_returnsZeroColonZeroZero() {
        assertEquals("0:00", formatMs(0))
        assertEquals("0:00", formatMs(-1))
    }

    @Test
    fun formatMs_positive_formatsMinutesAndSeconds() {
        assertEquals("0:05", formatMs(5_000))
        assertEquals("1:01", formatMs(61_000))
        assertEquals("10:00", formatMs(600_000))
    }
}
