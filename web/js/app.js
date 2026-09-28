/**
 * ProductHub — Storefront Frontend Application
 * Handles authentication, dynamic catalog, live cart, ACID checkout with idempotency,
 * and order history tracking.
 */

// ============================================================================
// Currency Formatting
// ============================================================================
function formatINR(amount) {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(Number(amount) || 0);
}

// Global Application State
const state = {
  user: null,
  token: localStorage.getItem('producthub_jwt') || null,
  categories: [],
  products: [],
  cart: { items: [], total: 0, subtotal: 0, tax: 0, shipping: 0 },
  selectedCategory: null,
  searchQuery: '',
  sortBy: 'created_at',
  sortOrder: 'desc',
  orders: [],
};

// ============================================================================
// API Client Helper
// ============================================================================
async function apiFetch(endpoint, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (state.token) {
    headers['Authorization'] = `Bearer ${state.token}`;
  }

  try {
    const res = await fetch(endpoint, { ...options, headers });
    const json = await res.json().catch(() => ({}));

    // Detect Cache Observability header
    const xCache = res.headers.get('X-Cache');
    const xIdemp = res.headers.get('X-Idempotency');

    if (!res.ok) {
      const errorMsg = json.error?.message || json.message || `Request failed (${res.status})`;
      throw new Error(errorMsg);
    }

    return { data: json.data, pagination: json.pagination, meta: { xCache, xIdemp } };
  } catch (err) {
    throw err;
  }
}

// Generate RFC-4122 v4 UUID for Idempotency-Key
function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

// ============================================================================
// Toast Notifications
// ============================================================================
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.innerHTML = `
    <span>${type === 'success' ? '✓' : type === 'error' ? '✕' : 'ℹ'}</span>
    <div>${message}</div>
  `;

  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    setTimeout(() => toast.remove(), 300);
  }, 3500);
}

// ============================================================================
// Authentication & User State
// ============================================================================
async function initAuth() {
  if (!state.token) {
    updateAuthUI();
    return;
  }

  try {
    const res = await apiFetch('/api/auth/me');
    state.user = res.data;
    updateAuthUI();
  } catch (err) {
    console.warn('Auth token expired or invalid:', err.message);
    logout();
  }
}

function updateAuthUI() {
  const authContainer = document.getElementById('auth-actions');
  if (!authContainer) return;

  if (state.user) {
    const adminLink = state.user.role === 'ADMIN'
      ? `<a href="/admin.html" class="nav-btn btn-primary" style="text-decoration:none; display:inline-flex; align-items:center; gap:0.35rem;">⚡ Admin Console</a>`
      : '';
    authContainer.innerHTML = `
      <div class="user-badge" id="user-chip">
        <span class="user-role-tag ${state.user.role === 'ADMIN' ? 'role-admin' : 'role-customer'}">${state.user.role}</span>
        <span>${state.user.name}</span>
      </div>
      ${adminLink}
      <button class="nav-btn btn-secondary" onclick="openOrdersModal()">My Orders</button>
      <button class="nav-btn btn-secondary" onclick="logout()">Sign Out</button>
    `;
  } else {
    authContainer.innerHTML = `
      <button class="nav-btn btn-primary" onclick="openAuthModal()">Sign In</button>
    `;
  }
}

async function loginUser(email, password) {
  try {
    const res = await apiFetch('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });

    state.token = res.data.token;
    state.user = res.data.user;
    localStorage.setItem('producthub_jwt', state.token);

    updateAuthUI();
    closeModal('auth-modal');
    showToast(`Welcome back, ${state.user.name}!`, 'success');

    // Load active user's cart
    fetchCart();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function registerUser(name, email, password) {
  try {
    const res = await apiFetch('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    });

    state.token = res.data.token;
    state.user = res.data.user;
    localStorage.setItem('producthub_jwt', state.token);

    updateAuthUI();
    closeModal('auth-modal');
    showToast(`Account registered successfully!`, 'success');

    fetchCart();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

function logout() {
  state.token = null;
  state.user = null;
  state.cart = { items: [], total: 0, subtotal: 0, tax: 0, shipping: 0 };
  localStorage.removeItem('producthub_jwt');
  updateAuthUI();
  updateCartBadge();
  showToast('You have been signed out.', 'info');
}

// ============================================================================
// Categories & Catalog
// ============================================================================
async function fetchCategories() {
  try {
    const res = await apiFetch('/api/categories');
    state.categories = res.data || [];
    renderCategories();
  } catch (err) {
    console.error('Failed to load categories:', err);
  }
}

function renderCategories() {
  const container = document.getElementById('category-chips');
  if (!container) return;

  const allChip = `<button class="category-chip ${state.selectedCategory === null ? 'active' : ''}" onclick="selectCategory(null)">All Items</button>`;
  const chips = state.categories.map(c => `
    <button class="category-chip ${state.selectedCategory === c.id ? 'active' : ''}" onclick="selectCategory(${c.id})">
      ${c.name}
    </button>
  `).join('');

  container.innerHTML = allChip + chips;
}

function selectCategory(catId) {
  state.selectedCategory = catId;
  renderCategories();
  fetchProducts();
}

async function fetchProducts() {
  const grid = document.getElementById('product-grid');
  if (!grid) return;

  grid.innerHTML = `<div style="grid-column: 1/-1; text-align: center; padding: 3rem; color: var(--text-muted);">
    Loading catalog...
  </div>`;

  try {
    let url = `/api/products?page=1&limit=24&sort=${state.sortBy}&order=${state.sortOrder}`;
    if (state.selectedCategory) {
      url += `&category_id=${state.selectedCategory}`;
    }
    if (state.searchQuery) {
      url += `&search=${encodeURIComponent(state.searchQuery)}`;
    }

    const res = await apiFetch(url);
    state.products = res.data || [];
    renderProducts();
  } catch (err) {
    grid.innerHTML = `<div style="grid-column: 1/-1; text-align: center; padding: 3rem; color: var(--danger);">
      Failed to load products: ${err.message}
    </div>`;
  }
}

function renderProducts() {
  const grid = document.getElementById('product-grid');
  if (!grid) return;

  if (state.products.length === 0) {
    grid.innerHTML = `
      <div style="grid-column: 1/-1; text-align: center; padding: 4rem 1rem; color: var(--text-muted);">
        <p style="font-size: 1.2rem; margin-bottom: 0.5rem;">No products match your search or filter</p>
        <button class="nav-btn btn-secondary" onclick="resetFilters()">Reset Filters</button>
      </div>
    `;
    return;
  }

  grid.innerHTML = state.products.map(p => {
    let stockClass = 'stock-in';
    let stockLabel = `${p.available_stock} in stock`;

    if (p.available_stock === 0) {
      stockClass = 'stock-out';
      stockLabel = 'Sold Out';
    } else if (p.available_stock <= 5) {
      stockClass = 'stock-low';
      stockLabel = `Only ${p.available_stock} left!`;
    }

    return `
      <div class="product-card" id="product-${p.id}">
        <div class="card-top">
          <span class="card-category">${p.category_name || 'Item'}</span>
          <span class="card-stock-badge ${stockClass}">
            <span class="dot"></span>
            ${stockLabel}
          </span>
        </div>

        <div class="product-image-placeholder">
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
          </svg>
        </div>

        <h3 class="product-title">${escapeHTML(p.name)}</h3>
        <div class="product-sku">SKU: ${p.sku}</div>
        <p class="product-desc">${escapeHTML(p.description || 'Premium quality product crafted with precision.')}</p>

        <div class="card-bottom">
          <div class="product-price">${formatINR(p.price)}</div>
          <button 
            class="btn-add-cart" 
            onclick="handleAddToCart(${p.id})" 
            ${p.available_stock === 0 ? 'disabled' : ''}>
            <span>🛒</span> Add to Cart
          </button>
        </div>
      </div>
    `;
  }).join('');
}

function resetFilters() {
  state.selectedCategory = null;
  state.searchQuery = '';
  document.getElementById('search-input').value = '';
  renderCategories();
  fetchProducts();
}

// ============================================================================
// Shopping Cart Operations
// ============================================================================
async function fetchCart() {
  if (!state.token) {
    updateCartBadge();
    return;
  }

  try {
    const res = await apiFetch('/api/cart');
    state.cart = res.data || { items: [] };
    updateCartBadge();
    renderCartDrawer();
  } catch (err) {
    console.warn('Cart sync issue:', err.message);
  }
}

function updateCartBadge() {
  const badge = document.getElementById('cart-badge');
  if (!badge) return;

  const count = (state.cart.items || []).reduce((acc, item) => acc + item.quantity, 0);
  badge.textContent = count;
  badge.style.display = count > 0 ? 'flex' : 'none';
}

async function handleAddToCart(productId) {
  if (!state.token) {
    openAuthModal();
    showToast('Please sign in to add items to your cart.', 'info');
    return;
  }

  try {
    await apiFetch('/api/cart/items', {
      method: 'POST',
      body: JSON.stringify({ product_id: productId, quantity: 1 }),
    });

    showToast('Item added to cart!', 'success');
    fetchCart();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function updateCartItemQuantity(productId, newQty) {
  if (newQty <= 0) {
    removeCartItem(productId);
    return;
  }

  try {
    await apiFetch(`/api/cart/items/${productId}`, {
      method: 'PUT',
      body: JSON.stringify({ quantity: newQty }),
    });
    fetchCart();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function removeCartItem(productId) {
  try {
    await apiFetch(`/api/cart/items/${productId}`, {
      method: 'DELETE',
    });
    showToast('Item removed from cart', 'info');
    fetchCart();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function clearCart() {
  try {
    await apiFetch('/api/cart', { method: 'DELETE' });
    showToast('Cart cleared', 'info');
    fetchCart();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

function renderCartDrawer() {
  const body = document.getElementById('cart-drawer-body');
  const footer = document.getElementById('cart-drawer-footer');
  if (!body || !footer) return;

  const items = state.cart.items || [];

  if (items.length === 0) {
    body.innerHTML = `
      <div class="cart-empty">
        <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M16 11V7a4 4 0 00-8 0v4M5 9h14l1 12H4L5 9z" />
        </svg>
        <p>Your shopping cart is currently empty</p>
      </div>
    `;
    footer.style.display = 'none';
    return;
  }

  footer.style.display = 'block';

  body.innerHTML = items.map(item => `
    <div class="cart-item" id="cart-item-${item.product_id}">
      <div class="cart-item-details">
        <div class="cart-item-title">${escapeHTML(item.product?.name || `Product #${item.product_id}`)}</div>
        <div class="cart-item-price">${formatINR(item.unit_price)} &times; ${item.quantity} = ${formatINR(item.subtotal)}</div>
      </div>
      <div class="quantity-stepper">
        <button class="step-btn" onclick="updateCartItemQuantity(${item.product_id}, ${item.quantity - 1})">-</button>
        <span class="step-val">${item.quantity}</span>
        <button class="step-btn" onclick="updateCartItemQuantity(${item.product_id}, ${item.quantity + 1})">+</button>
      </div>
      <button class="close-btn" onclick="removeCartItem(${item.product_id})" title="Remove">✕</button>
    </div>
  `).join('');

  document.getElementById('cart-subtotal').textContent = formatINR(state.cart.subtotal);
  document.getElementById('cart-tax').textContent = formatINR(state.cart.tax);
  document.getElementById('cart-grand-total').textContent = formatINR(state.cart.total_amount);
}

// ============================================================================
// ACID Checkout & Idempotency
// ============================================================================
function openCheckoutModal() {
  if (!state.token) {
    openAuthModal();
    return;
  }
  if (!state.cart.items || state.cart.items.length === 0) {
    showToast('Your cart is empty!', 'error');
    return;
  }

  // Pre-generate unique client idempotency key
  const idempKey = generateUUID();
  document.getElementById('checkout-idemp-key').value = idempKey;
  document.getElementById('checkout-total-display').textContent = formatINR(state.cart.total_amount);

  closeCartDrawer();
  openModal('checkout-modal');
}

async function executeCheckout(e) {
  e.preventDefault();

  const address = document.getElementById('checkout-address').value.trim();
  const paymentMethod = document.getElementById('checkout-payment').value;
  const idempKey = document.getElementById('checkout-idemp-key').value;
  const btn = document.getElementById('btn-submit-order');

  if (!address) {
    showToast('Please provide a valid delivery address', 'error');
    return;
  }

  btn.disabled = true;
  btn.innerHTML = `<span class="spinner"></span> Processing Transaction...`;

  try {
    const res = await apiFetch('/api/orders', {
      method: 'POST',
      headers: {
        'Idempotency-Key': idempKey,
      },
      body: JSON.stringify({
        shipping_address: address,
        payment_method: paymentMethod,
      }),
    });

    closeModal('checkout-modal');
    showToast(`Order #${res.data.id} placed successfully!`, 'success');

    // Display confirmation modal
    showOrderConfirmation(res.data, res.meta?.xIdemp);

    // Refresh cart & products (stock changed)
    fetchCart();
    fetchProducts();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = `Place Order`;
  }
}

function showOrderConfirmation(order, idempHeader) {
  const content = document.getElementById('confirmation-details');
  if (!content) return;

  content.innerHTML = `
    <div style="text-align: center; margin-bottom: 1.5rem;">
      <div style="width: 56px; height: 56px; border-radius: 50%; background: rgba(16, 185, 129, 0.2); color: #34d399; font-size: 2rem; display: flex; align-items: center; justify-content: center; margin: 0 auto 1rem;">✓</div>
      <h3 style="font-size: 1.4rem; margin-bottom: 0.25rem;">Order Confirmed!</h3>
      <p style="color: var(--text-muted); font-size: 0.9rem;">Order ID: #${order.id} | Status: <strong style="color: #60a5fa;">${order.status}</strong></p>
      ${idempHeader === 'HIT' ? '<p style="color: #a78bfa; font-size: 0.8rem; margin-top: 0.25rem;">⚡ Idempotent Replay Verified</p>' : ''}
    </div>

    <div style="background: rgba(255,255,255,0.03); border: 1px solid var(--border-subtle); border-radius: 8px; padding: 1rem; margin-bottom: 1.25rem;">
      <div class="cost-row"><span>Total Paid</span><strong style="color: #fff;">${formatINR(order.total_amount)}</strong></div>
      <div class="cost-row"><span>Items Count</span><span>${(order.items || []).length} products</span></div>
      <div class="cost-row"><span>Date</span><span>${new Date(order.created_at).toLocaleString()}</span></div>
    </div>
  `;

  openModal('confirmation-modal');
}

// ============================================================================
// Order Tracking & Cancellation
// ============================================================================
async function openOrdersModal() {
  if (!state.token) {
    openAuthModal();
    return;
  }

  const list = document.getElementById('orders-list');
  if (!list) return;

  list.innerHTML = `<div style="text-align: center; padding: 2rem; color: var(--text-muted);">Loading your orders...</div>`;
  openModal('orders-modal');

  try {
    const res = await apiFetch('/api/orders?page=1&limit=20');
    state.orders = res.data || [];
    renderOrdersList();
  } catch (err) {
    list.innerHTML = `<div style="color: var(--danger); text-align: center; padding: 2rem;">${err.message}</div>`;
  }
}

function renderOrdersList() {
  const list = document.getElementById('orders-list');
  if (!list) return;

  if (state.orders.length === 0) {
    list.innerHTML = `<div style="text-align: center; padding: 3rem; color: var(--text-muted);">No orders found. Start shopping today!</div>`;
    return;
  }

  list.innerHTML = state.orders.map(o => {
    const statusClass = `status-${o.status.toLowerCase()}`;
    const canCancel = o.status === 'PENDING' || o.status === 'CONFIRMED';

    return `
      <div class="order-card" id="order-${o.id}">
        <div class="order-header">
          <div>
            <strong>Order #${o.id}</strong>
            <span style="color: var(--text-muted); font-size: 0.82rem; margin-left: 0.5rem;">${new Date(o.created_at).toLocaleDateString()}</span>
          </div>
          <span class="order-status-pill ${statusClass}">${o.status}</span>
        </div>

        <div style="font-size: 0.88rem; color: var(--text-secondary); margin-bottom: 0.75rem;">
          ${(o.items || []).map(i => `${escapeHTML(i.product?.name || `Product #${i.product_id}`)} &times; ${i.quantity}`).join(', ')}
        </div>

        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div style="font-size: 1.1rem; font-weight: 700; color: #fff;">${formatINR(o.total_amount)}</div>
          ${canCancel ? `<button class="nav-btn btn-secondary" style="color: var(--danger); border-color: rgba(239, 68, 68, 0.4); font-size: 0.78rem; padding: 0.35rem 0.75rem;" onclick="cancelOrder(${o.id})">Cancel Order</button>` : ''}
        </div>
      </div>
    `;
  }).join('');
}

async function cancelOrder(orderId) {
  if (!confirm(`Are you sure you want to cancel Order #${orderId}? Stock will be released back to inventory.`)) {
    return;
  }

  try {
    await apiFetch(`/api/orders/${orderId}/cancel`, { method: 'POST' });
    showToast(`Order #${orderId} has been cancelled.`, 'info');
    openOrdersModal();
    fetchProducts();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// ============================================================================
// Modal & Drawer Utilities
// ============================================================================
function openModal(id) {
  const el = document.getElementById(id);
  if (el) el.classList.add('active');
}

function closeModal(id) {
  const el = document.getElementById(id);
  if (el) el.classList.remove('active');
}

function openCartDrawer() {
  const el = document.getElementById('cart-drawer-overlay');
  if (el) el.classList.add('active');
}

function closeCartDrawer() {
  const el = document.getElementById('cart-drawer-overlay');
  if (el) el.classList.remove('active');
}

function openAuthModal() {
  openModal('auth-modal');
}

function setDemoCredentials(role) {
  const emailInput = document.getElementById('auth-email');
  const passInput = document.getElementById('auth-password');

  if (role === 'admin') {
    emailInput.value = 'admin@producthub.local';
    passInput.value = 'admin123';
  } else {
    emailInput.value = 'john@example.com';
    passInput.value = 'password123';
  }
}

function escapeHTML(str) {
  if (!str) return '';
  return str.replace(/[&<>'"]/g,
    tag => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[tag] || tag)
  );
}

// ============================================================================
// Application Startup
// ============================================================================
document.addEventListener('DOMContentLoaded', () => {
  initAuth();
  fetchCategories();
  fetchProducts();
  fetchCart();

  // Search input live filter with debounce
  const searchInput = document.getElementById('search-input');
  if (searchInput) {
    let timeout = null;
    searchInput.addEventListener('input', (e) => {
      clearTimeout(timeout);
      timeout = setTimeout(() => {
        state.searchQuery = e.target.value.trim();
        fetchProducts();
      }, 350);
    });
  }

  // Sort dropdown
  const sortSelect = document.getElementById('sort-select');
  if (sortSelect) {
    sortSelect.addEventListener('change', (e) => {
      const [by, order] = e.target.value.split(':');
      state.sortBy = by;
      state.sortOrder = order;
      fetchProducts();
    });
  }

  // Auth Form Submit
  const authForm = document.getElementById('auth-form');
  if (authForm) {
    authForm.addEventListener('submit', (e) => {
      e.preventDefault();
      const email = document.getElementById('auth-email').value.trim();
      const pass = document.getElementById('auth-password').value;
      const isRegister = document.getElementById('auth-is-register').checked;
      const name = document.getElementById('auth-name').value.trim();

      if (isRegister) {
        registerUser(name, email, pass);
      } else {
        loginUser(email, pass);
      }
    });
  }

  // Checkout Form Submit
  const checkoutForm = document.getElementById('checkout-form');
  if (checkoutForm) {
    checkoutForm.addEventListener('submit', executeCheckout);
  }
});
