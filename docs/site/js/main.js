/* ============================================
   EasyWidge — Main JavaScript
   ============================================ */

(function () {
  "use strict";

  // --- Navbar scroll effect ---
  const navbar = document.querySelector(".navbar");
  if (navbar) {
    const onScroll = () => {
      navbar.classList.toggle("scrolled", window.scrollY > 40);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
  }

  // --- Mobile nav toggle ---
  const toggle = document.querySelector(".nav-toggle");
  const navLinks = document.querySelector(".nav-links");
  if (toggle && navLinks) {
    toggle.addEventListener("click", () => {
      const isOpen = navLinks.classList.toggle("open");
      toggle.setAttribute("aria-expanded", isOpen);
    });

    // Close menu when a link is clicked
    navLinks.querySelectorAll("a").forEach((link) => {
      link.addEventListener("click", () => {
        navLinks.classList.remove("open");
        toggle.setAttribute("aria-expanded", "false");
      });
    });

    // Close menu on outside click
    document.addEventListener("click", (e) => {
      if (!toggle.contains(e.target) && !navLinks.contains(e.target)) {
        navLinks.classList.remove("open");
        toggle.setAttribute("aria-expanded", "false");
      }
    });
  }

  // --- Scroll reveal ---
  const revealElements = document.querySelectorAll("[data-reveal]");
  if (revealElements.length > 0 && "IntersectionObserver" in window) {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("revealed");
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.1, rootMargin: "0px 0px -40px 0px" }
    );

    revealElements.forEach((el) => observer.observe(el));
  }

  // --- Smooth scroll for anchor links ---
  document.querySelectorAll('a[href^="#"]').forEach((anchor) => {
    anchor.addEventListener("click", (e) => {
      const targetId = anchor.getAttribute("href");
      if (targetId === "#") return;
      const target = document.querySelector(targetId);
      if (target) {
        e.preventDefault();
        target.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    });
  });

  // --- Fetch current version ---
  const versionUrl = "https://raw.githubusercontent.com/gcclinux/WeatherWidget/refs/heads/main/release";
  const versionElement = document.getElementById("app-version");
  if (versionElement) {
    fetch(versionUrl)
      .then(response => response.text())
      .then(version => {
        if (version) {
          versionElement.textContent = `v${version.trim()}`;
        }
      })
      .catch(err => {
        console.error("Failed to fetch version:", err);
      });
  }
})();
