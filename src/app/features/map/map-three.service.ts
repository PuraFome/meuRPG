import { Injectable, inject, NgZone } from '@angular/core';

// Marker data interface — exported for the component.
export interface Marker3D {
  lon: number;
  lat: number;
  label?: string;
  color?: string;
}

/**
 * Three.js wrapper service for the 2.5D isometric map view.
 *
 * Three.js is loaded via dynamic `import()` — it never appears in the
 * initial application bundle.  All type declarations use inline `typeof
 * import(...)` so they are erased at compile time and have zero runtime
 * cost.
 */
@Injectable({ providedIn: 'root' })
export class MapThreeService {
  // ── Stores for dynamically imported modules (typed via `any` to avoid
  //    complex type gymnastics with inline `typeof import(...)` patterns
  //    that confuse the Angular template compiler). ──
  private _THREE: typeof import('three') | null = null;
  private _OrbitControls: typeof import('three/examples/jsm/controls/OrbitControls.js').OrbitControls | null = null;

  private scene: import('three').Scene | null = null;
  private camera: import('three').OrthographicCamera | null = null;
  private renderer: import('three').WebGLRenderer | null = null;
  private controls: import('three/examples/jsm/controls/OrbitControls.js').OrbitControls | null = null;
  private grid: import('three').GridHelper | null = null;
  private mapPlane: import('three').Mesh | null = null;
  private markerMeshes: import('three').Mesh[] = [];

  private container: HTMLElement | null = null;
  private animationId: number | null = null;
  private isActive = false;
  private initialized = false;
  private initPromise: Promise<void> | null = null;

  private centerLon = 0;
  private centerLat = 0;
  private scale = 2000;

  private readonly zone = inject(NgZone);

  // ─────────────────────────────────────────────────────────────────
  //  Internal helpers
  // ─────────────────────────────────────────────────────────────────

  private async loadDeps(): Promise<{ THREE: typeof import('three'); OrbitControls: typeof import('three/examples/jsm/controls/OrbitControls.js').OrbitControls }> {
    if (this._THREE && this._OrbitControls) {
      return { THREE: this._THREE, OrbitControls: this._OrbitControls };
    }
    const [THREE, { OrbitControls }] = await Promise.all([
      import('three'),
      import('three/examples/jsm/controls/OrbitControls.js'),
    ]);
    this._THREE = THREE;
    this._OrbitControls = OrbitControls;
    return { THREE, OrbitControls };
  }

  // ─────────────────────────────────────────────────────────────────
  //  Public API
  // ─────────────────────────────────────────────────────────────────

  /**
   * Initialises the Three.js scene, camera, renderer and controls.
   * Must be called once before `activate()`. Safe to call multiple times.
   */
  async init(container: HTMLElement): Promise<void> {
    if (this.initialized) return;
    if (this.initPromise) return this.initPromise;

    this.container = container;

    this.initPromise = this.zone.runOutsideAngular(async () => {
      const { THREE, OrbitControls } = await this.loadDeps();

      // Scene
      const scene = new THREE.Scene();
      scene.background = new THREE.Color(0x1a1a2e);
      this.scene = scene;

      // Isometric orthographic camera
      const aspect = container.clientWidth / container.clientHeight || 16 / 9;
      const d = 1000;
      const camera = new THREE.OrthographicCamera(
        -d * aspect, d * aspect, d, -d, 0.1, 5000,
      );
      camera.position.set(1, 0.6, 1).normalize().multiplyScalar(1200);
      camera.lookAt(0, 0, 0);
      this.camera = camera;

      // Renderer
      const renderer = new THREE.WebGLRenderer({ antialias: true });
      renderer.setSize(container.clientWidth, container.clientHeight);
      renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
      container.appendChild(renderer.domElement);
      this.renderer = renderer;

      // Orbit controls
      const controls = new OrbitControls(camera, renderer.domElement);
      controls.enableDamping = true;
      controls.dampingFactor = 0.08;
      controls.target.set(0, 0, 0);
      controls.minZoom = 0.3;
      controls.maxZoom = 8;
      controls.update();
      this.controls = controls;

      // Grid helper (XZ plane)
      const grid = new THREE.GridHelper(2400, 48, 0x4a4a8a, 0x3a3a6a);
      grid.position.y = -1;
      scene.add(grid);
      this.grid = grid;

      // Lights
      scene.add(new THREE.AmbientLight(0xffffff, 0.55));
      const dirLight = new THREE.DirectionalLight(0xffffff, 0.9);
      dirLight.position.set(200, 400, 200);
      scene.add(dirLight);

      this.initialized = true;
    });

    return this.initPromise;
  }

  /**
   * Switches the view to 2.5D mode.
   *
   * @param olCanvas  The OpenLayers canvas whose content becomes the map texture.
   * @param center    Map centre in [lon, lat].
   * @param zoom      Current zoom level (used to calibrate unit scale).
   */
  async activate(
    olCanvas: HTMLCanvasElement | null,
    center: [number, number],
    zoom: number,
  ): Promise<void> {
    if (!this.container) return;
    if (!this.initialized) await this.init(this.container);
    if (this.isActive) return;

    this.isActive = true;
    this.container.style.display = 'block';

    const THREE = this._THREE!;

    this.centerLon = center[0];
    this.centerLat = center[1];
    this.scale = Math.max(500, 2400 / (Math.pow(2, 14 - zoom) * 0.5 || 1));

    this.clearMapPlane();

    if (olCanvas) {
      const texture = new THREE.CanvasTexture(olCanvas);
      texture.needsUpdate = true;

      const aspect = olCanvas.width / olCanvas.height || 1;
      const w = 2400;
      const h = 2400 / aspect;

      const geometry = new THREE.PlaneGeometry(w, h);
      const material = new THREE.MeshBasicMaterial({
        map: texture,
        side: THREE.DoubleSide,
      });
      const plane = new THREE.Mesh(geometry, material);
      plane.rotation.x = -Math.PI / 2;
      plane.position.y = 0;
      this.scene!.add(plane);
      this.mapPlane = plane;
    }

    this.startAnimation();
  }

  /** Hides the 3D view and stops the render loop. */
  deactivate(): void {
    this.isActive = false;
    this.stopAnimation();
    if (this.container) {
      this.container.style.display = 'none';
    }
  }

  // ── Grid ──

  /** Creates a visible grid on the 3D floor. */
  renderGrid(): void {
    if (this.grid) return;
    if (!this._THREE) return;
    const THREE = this._THREE;
    const grid = new THREE.GridHelper(2400, 48, 0x4a4a8a, 0x3a3a6a);
    grid.position.y = -1;
    this.scene!.add(grid);
    this.grid = grid;
  }

  // ── Markers ──

  /** Renders markers as small 3D spheres floating above the plane. */
  renderMarkers(markers: Marker3D[]): void {
    this.clearMarkers();
    if (!this._THREE || !this.scene) return;
    const THREE = this._THREE;

    for (const m of markers) {
      const [x, z] = this.olToThree(m.lon, m.lat);
      const color = m.color || '#ff6b6b';

      const sphere = new THREE.Mesh(
        new THREE.SphereGeometry(12, 12, 12),
        new THREE.MeshStandardMaterial({
          color: new THREE.Color(color),
          emissive: new THREE.Color(color),
          emissiveIntensity: 0.3,
        }),
      );
      sphere.position.set(x, 8, z);
      this.scene.add(sphere);
      this.markerMeshes.push(sphere);
    }
  }

  // ── Lifecycle ──

  /** Call when the container resizes so the renderer & camera match. */
  resize(): void {
    if (!this.container || !this.renderer || !this.camera) return;

    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    const aspect = width / height;

    const d = 1000;
    this.camera.left = -d * aspect;
    this.camera.right = d * aspect;
    this.camera.top = d;
    this.camera.bottom = -d;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height);
  }

  /** Releases all Three.js resources. Call on component destroy. */
  destroy(): void {
    this.deactivate();
    this.clearMarkers();
    this.clearMapPlane();

    if (this.renderer) {
      this.renderer.dispose();
      if (this.renderer.domElement.parentNode) {
        this.renderer.domElement.parentNode.removeChild(this.renderer.domElement);
      }
    }

    this.scene?.traverse((child: import('three').Object3D) => {
      const mesh = child as import('three').Mesh;
      if (mesh.geometry) mesh.geometry.dispose();
      if (mesh.material) {
        if (Array.isArray(mesh.material)) {
          mesh.material.forEach((m: import('three').Material) => m.dispose());
        } else {
          mesh.material.dispose();
        }
      }
    });

    this.scene = null;
    this.camera = null;
    this.renderer = null;
    this.controls = null;
    this.grid = null;
    this.mapPlane = null;
    this.markerMeshes = [];
    this.initialized = false;
    this.initPromise = null;
    this.isActive = false;
    this.container = null;
  }

  /** Returns true while the 3D view is active. */
  get active(): boolean {
    return this.isActive;
  }

  /** Returns the current zoom level the camera was last set to. */
  getZoom(): number {
    return this.controls
      ? Math.round(
          this.controls.object instanceof this._THREE!.OrthographicCamera
            ? (this.controls.object as import('three').OrthographicCamera).zoom
            : 10,
        )
      : 10;
  }

  // ── Private helpers ──

  private startAnimation(): void {
    const animate = () => {
      if (!this.isActive) return;
      this.controls?.update();
      if (this.scene && this.camera && this.renderer) {
        this.renderer.render(this.scene, this.camera);
      }
      this.animationId = requestAnimationFrame(animate);
    };
    this.animationId = requestAnimationFrame(animate);
  }

  private stopAnimation(): void {
    if (this.animationId !== null) {
      cancelAnimationFrame(this.animationId);
      this.animationId = null;
    }
  }

  private clearMapPlane(): void {
    if (this.mapPlane && this.scene) {
      this.scene.remove(this.mapPlane);
      this.mapPlane.geometry.dispose();
      if (Array.isArray(this.mapPlane.material)) {
        this.mapPlane.material.forEach((m: import('three').Material) => m.dispose());
      } else {
        this.mapPlane.material.dispose();
      }
      this.mapPlane = null;
    }
  }

  private clearMarkers(): void {
    if (!this.scene) return;
    for (const m of this.markerMeshes) {
      this.scene.remove(m);
      m.geometry.dispose();
      if (Array.isArray(m.material)) {
        m.material.forEach((mat: import('three').Material) => mat.dispose());
      } else {
        m.material.dispose();
      }
    }
    this.markerMeshes = [];
  }

  /** Converts OpenLayers [lon, lat] to Three.js [x, z] relative to the map centre. */
  private olToThree(lon: number, lat: number): [number, number] {
    const dx = (lon - this.centerLon) * this.scale;
    const dy = (lat - this.centerLat) * this.scale;
    return [dx, -dy];
  }
}
