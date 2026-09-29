/**
 * ProductHub — Operations Console & Admin Dashboard Application
 * Handles authentication gate, real-time KPI aggregation, product CRUD,
 * inventory restock, order state transitions, and background worker telemetry.
 */

// Global Admin Application State
const adminState = {
  user: null,
  token: localStorage.getItem('producthub_jwt') || null,
  categories: [],
  products: [],
  orders: [],
  workerStats: null,
  inventorySummary: null,
  currentTab: 'overview',
  workerPollInterval: null,
};
function formatINR(amount) {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(Number(amount) || 0);
}
// ============================================================================
// API Client Helper
// ============================================================================
async function adminApiFetch(endpoint, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (adminState.token) {
    headers['Authorization'] = `Bearer ${adminState.token}`;
  }

  try {
    const res = await fetch(endpoint, { ...options, headers });
    const json = await res.json().catch(() => ({}));

    if (!res.ok) {
      const errorMsg = json.error?.message || json.message || `Request failed (${res.status})`;
      throw new Error(errorMsg);
    }

    return { data: json.data, pagination: json.pagination };
  } catch (err) {
    throw err;
  }
}

// ============================================================================
// Toast Notifications
// ============================================================================
function showAdminToast(message, type = 'info') {
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
// Modal Control Helpers
// ============================================================================
function openAdminModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) modal.classList.add('active');
}

function closeAdminModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) modal.classList.remove('active');
}

// ============================================================================
// Authentication & Role Verification Gate
// ============================================================================
async function verifyAdminAuth() {
  const gate = document.getElementById('admin-auth-gate');
  const userChip = document.getElementById('admin-user-name');

  if (!adminState.token) {
    if (gate) gate.style.display = 'flex';
    return false;
  }

  try {
    const res = await adminApiFetch('/api/auth/me');
    const user = res.data;

    if (user.role !== 'ADMIN') {
      showAdminToast('Access Denied: Your account does not possess ADMIN privileges.', 'error');
      if (gate) gate.style.display = 'flex';
      return false;
    }

    adminState.user = user;
    if (userChip) userChip.textContent = user.name || 'Admin';
    if (gate) gate.style.display = 'none';
    return true;
  } catch (err) {
    console.warn('Admin token verification failed:', err.message);
    if (gate) gate.style.display = 'flex';
    return false;
  }
}

function fillAdminCredentials() {
  document.getElementById('gate-email').value = 'admin@example.com';
  document.getElementById('gate-password').value = 'Admin@123';
}

async function handleAdminLogin(email, password) {
  try {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });

    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.error?.message || 'Login failed');
    }

    if (json.data.user?.role !== 'ADMIN') {
      throw new Error('This account does not have administrator privileges.');
    }

    adminState.token = json.data.token;
    adminState.user = json.data.user;
    localStorage.setItem('producthub_jwt', adminState.token);

    document.getElementById('admin-auth-gate').style.display = 'none';
    document.getElementById('admin-user-name').textContent = adminState.user.name;
    showAdminToast(`Authenticated as ${adminState.user.name}`, 'success');

    bootDashboard();
  } catch (err) {
    showAdminToast(err.message, 'error');
  }
}

function adminLogout() {
  localStorage.removeItem('producthub_jwt');
  adminState.token = null;
  adminState.user = null;
  window.location.href = '/';
}

// ============================================================================
// Tab Navigation
// ============================================================================
function switchTab(tabName) {
  adminState.currentTab = tabName;

  // Update tab buttons
  const tabs = document.querySelectorAll('.admin-tab');
  tabs.forEach((tab) => {
    tab.classList.toggle('active', tab.getAttribute('onclick')?.includes(tabName));
  });

  // Update tab views
  const views = document.querySelectorAll('.admin-view');
  views.forEach((view) => {
    view.classList.toggle('active', view.id === `view-${tabName}`);
  });

  // Load relevant data
  if (tabName === 'overview') loadOverviewKPIs();
  if (tabName === 'products') loadProductsTable();
  if (tabName === 'inventory') loadInventoryTable();
  if (tabName === 'orders') loadOrdersTable();
  if (tabName === 'workers') loadWorkerStats();
}

function refreshCurrentView() {
  switchTab(adminState.currentTab);
  showAdminToast('Dashboard data refreshed', 'info');
}

// ============================================================================
// 1. Overview & Real-Time KPIs
// ============================================================================
async function loadOverviewKPIs() {
  try {
    const [invRes, prodRes, workerRes] = await Promise.all([
      adminApiFetch('/api/inventory/summary').catch(() => ({ data: {} })),
      adminApiFetch('/api/products?limit=100').catch(() => ({ data: [] })),
      adminApiFetch('/api/admin/workers/stats').catch(() => ({ data: {} })),
    ]);

    const summary = invRes.data || {};
    const products = prodRes.data || [];
    const workers = workerRes.data || {};

    adminState.inventorySummary = summary;
    adminState.products = products;
    adminState.workerStats = workers;

    // Update KPI card numbers
    document.getElementById('kpi-total-products').textContent = products.length || summary.total_products || '0';
    document.getElementById('kpi-low-stock').textContent = summary.low_stock_count ?? '0';
    document.getElementById('kpi-total-stock').textContent = (summary.total_stock ?? 0).toLocaleString();
    document.getElementById('kpi-worker-queue').textContent = `${workers.queue_depth ?? 0} / ${workers.queue_capacity ?? 100}`;

    // Update worker status box
    document.getElementById('overview-worker-status').textContent = workers.status || 'ONLINE';
    document.getElementById('overview-active-workers').textContent = `${workers.total_workers || 4} Goroutines`;
    document.getElementById('overview-processed-tasks').textContent = (workers.total_processed_tasks ?? 0).toLocaleString();
    document.getElementById('overview-failed-tasks').textContent = (workers.total_failed_tasks ?? 0).toLocaleString();

    // Populate low stock alerts table in Overview
    const lowStockItems = products.filter((p) => p.stock <= (p.low_stock_threshold || 10));
    const tbody = document.getElementById('overview-low-stock-tbody');
    if (tbody) {
      if (lowStockItems.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--success); padding: 1.5rem;">✓ All inventory items are adequately stocked above threshold.</td></tr>`;
      } else {
        tbody.innerHTML = lowStockItems.map((p) => `
          <tr>
            <td>
              <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(p.name)}</div>
              <div style="font-size: 0.78rem; color: var(--text-muted);">${escapeHtml(p.category?.name || 'General')}</div>
            </td>
            <td><code>${escapeHtml(p.sku)}</code></td>
            <td>
              <span class="stock-badge ${p.stock === 0 ? 'badge-out' : 'badge-low'}">
                ${p.stock === 0 ? 'Out of Stock (0)' : `${p.stock} units left`}
              </span>
            </td>
            <td>${p.low_stock_threshold || 10}</td>
            <td>
              <button class="btn-action btn-success" onclick="openRestockForProduct(${p.id})">
                + Restock
              </button>
            </td>
          </tr>
        `).join('');
      }
    }
  } catch (err) {
    console.error('Failed to load overview KPIs:', err);
  }
}

// ============================================================================
// 2. Product Catalog Management
// ============================================================================
async function loadProductsTable() {
  const tbody = document.getElementById('admin-products-tbody');
  if (!tbody) return;

  try {
    const searchVal = document.getElementById('admin-product-search')?.value.trim() || '';
    const catVal = document.getElementById('admin-product-cat-filter')?.value || '';

    let url = `/api/products?limit=100`;
    if (searchVal) url += `&search=${encodeURIComponent(searchVal)}`;
    if (catVal) url += `&category_id=${encodeURIComponent(catVal)}`;

    const res = await adminApiFetch(url);
    adminState.products = res.data || [];

    if (adminState.products.length === 0) {
      tbody.innerHTML = `<tr><td colspan="9" style="text-align: center; color: var(--text-muted); padding: 2rem;">No products found matching criteria.</td></tr>`;
      return;
    }

    tbody.innerHTML = adminState.products.map((p) => {
      const threshold = p.low_stock_threshold || 10;
      const isOut = p.stock === 0;
      const isLow = !isOut && p.stock <= threshold;

      let stockBadge;
      let stockLabel;

      if (isOut) {
        stockBadge = 'badge-out';
        stockLabel = 'OUT OF STOCK';
      } else if (isLow) {
        stockBadge = 'badge-low';
        stockLabel = `LOW STOCK — ${p.stock} units`;
      } else {
        stockBadge = 'badge-in';
        stockLabel = `HEALTHY — ${p.stock} units`;
      }

      return `
        <tr>
          <td>#${p.id}</td>
         <td>
  <div style="font-weight: 600; color: var(--text-primary);">
    ${escapeHtml(p.name)}
  </div>

  <div style="font-size: 0.78rem; color: var(--text-muted); max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
    ${escapeHtml(p.description || '')}
  </div>
</td>

<td>
  ${p.image_url
          ? `<img
          src="${escapeHtml(p.image_url)}"
          alt="${escapeHtml(p.name)}"
          class="admin-product-thumb"
          onerror="this.style.display='none';"
        >`
          : `<div class="admin-product-thumb-placeholder">No Image</div>`
        }
</td>

<td>
  <code>${escapeHtml(p.sku)}</code>
</td>
          <td><span class="category-chip" style="font-size: 0.75rem; padding: 0.2rem 0.5rem;">${escapeHtml(p.category?.name || 'General')}</span></td>
          <td style="font-weight: 600; color: var(--text-primary);">
  ${formatINR(p.price)}
</td>
          <td>
  <span class="stock-badge ${stockBadge}">
    ${stockLabel}
  </span>

  ${!isOut
          ? `<div style="font-size: 0.7rem; color: var(--text-muted); margin-top: 0.25rem;">
          Threshold: ${threshold}
        </div>`
          : ''
        }
</td>
<td>
  <span style="font-size: 0.8rem; font-weight: 600; color: ${p.status === 'ACTIVE' ? 'var(--success)' : 'var(--danger)'};">
    ${p.status === 'ACTIVE' ? 'Active' : 'Inactive'}
  </span>
</td>
<td>
  <div style="display: flex; gap: 0.4rem;">
    <button class="btn-action" onclick="openEditProductModal(${p.id})">Edit</button>
    <button
      class="btn-action ${p.status === 'ACTIVE' ? 'btn-danger' : 'btn-success'}"
      onclick="toggleProductActive(${p.id}, ${p.status === 'ACTIVE'})"
    >
      ${p.status === 'ACTIVE' ? 'Deactivate' : 'Activate'}
    </button>
  </div>
</td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" style="text-align: center; color: var(--danger); padding: 2rem;">Failed to load products: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function openCreateProductModal() {
  document.getElementById('product-modal-title').textContent = 'Add New Product';
  document.getElementById('prod-id').value = '';
  document.getElementById('prod-title').value = '';
  document.getElementById('prod-sku').value = '';
  document.getElementById('prod-description').value = '';
  document.getElementById('prod-price').value = '';
  document.getElementById('prod-stock').value = '50';
  document.getElementById('prod-threshold').value = '10';

  populateCategoriesDropdown('prod-category');
  openAdminModal('product-modal');
}

function openEditProductModal(productId) {
  const p = adminState.products.find((item) => item.id === productId);
  if (!p) return;

  document.getElementById('product-modal-title').textContent = `Edit Product #${p.id}`;
  document.getElementById('prod-id').value = p.id;
  document.getElementById('prod-title').value = p.name;
  document.getElementById('prod-sku').value = p.sku;
  document.getElementById('prod-description').value = p.description || '';
  document.getElementById('prod-price').value = p.price;
  document.getElementById('prod-stock').value = p.stock;
  document.getElementById('prod-threshold').value = p.low_stock_threshold || 10;

  populateCategoriesDropdown('prod-category', p.category_id);
  openAdminModal('product-modal');
}

async function saveProductForm(e) {
  e.preventDefault();
  const id = document.getElementById('prod-id').value;
  const title = document.getElementById('prod-title').value.trim();
  const sku = document.getElementById('prod-sku').value.trim();
  const category_id = parseInt(document.getElementById('prod-category').value, 10);
  const description = document.getElementById('prod-description').value.trim();
  const price = parseFloat(document.getElementById('prod-price').value);
  const stock = parseInt(document.getElementById('prod-stock').value, 10);
  const low_stock_threshold = parseInt(document.getElementById('prod-threshold').value, 10);

  const payload = {
    name: title,
    sku,
    category_id,
    description,
    price,
    stock,
  };

  try {
    if (id) {
      // Update Product
      await adminApiFetch(`/api/products/${id}`, {
        method: 'PUT',
        body: JSON.stringify(payload),
      });
      showAdminToast(`Product #${id} successfully updated!`, 'success');
    } else {
      // Create Product
      await adminApiFetch('/api/products', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      showAdminToast('New product created successfully!', 'success');
    }

    closeAdminModal('product-modal');
    loadProductsTable();
    loadOverviewKPIs();
  } catch (err) {
    showAdminToast(err.message, 'error');
  }
}

async function toggleProductActive(productId, currentlyActive) {
  const actionName = currentlyActive ? 'deactivate' : 'activate';
  if (!confirm(`Are you sure you want to ${actionName} product #${productId}?`)) return;

  try {
    if (currentlyActive) {
      await adminApiFetch(`/api/products/${productId}`, { method: 'DELETE' });
      showAdminToast(`Product #${productId} has been deactivated`, 'info');
    } else {
      await adminApiFetch(`/api/products/${productId}`, {
        method: 'PUT',
        body: JSON.stringify({ status: 'ACTIVE' }),
      });

      showAdminToast(`Product #${productId} has been reactivated`, 'success');
    }

    loadProductsTable();
    loadOverviewKPIs();
  } catch (err) {
    showAdminToast(err.message, 'error');
  }
}

// ============================================================================
// 3. Inventory & Restock
// ============================================================================
async function loadInventoryTable() {
  const tbody = document.getElementById('admin-inventory-tbody');
  if (!tbody) return;

  try {
    const res = await adminApiFetch('/api/products?limit=100');
    adminState.products = res.data || [];

    tbody.innerHTML = adminState.products.map((p) => {
      const threshold = p.low_stock_threshold || 10;
      let statusBadge = '<span class="stock-badge badge-in">HEALTHY</span>';
      if (p.stock === 0) {
        statusBadge = '<span class="stock-badge badge-out">OUT OF STOCK</span>';
      } else if (p.stock <= threshold) {
        statusBadge = '<span class="stock-badge badge-low">LOW STOCK ALERT</span>';
      }

      return `
        <tr>
          <td>
  <div style="display: flex; align-items: center; gap: 0.75rem;">
    ${p.image_url
          ? `<img
            src="${escapeHtml(p.image_url)}"
            alt="${escapeHtml(p.name)}"
            style="width: 50px; height: 50px; object-fit: cover; border-radius: 8px; border: 1px solid var(--border);"
            onerror="this.style.display='none';"
          >`
          : `<div style="width: 50px; height: 50px; border-radius: 8px; background: var(--bg-secondary); display: flex; align-items: center; justify-content: center; color: var(--text-muted); font-size: 0.7rem;">
            No image
          </div>`
        }

    <div>
      <div style="font-weight: 600; color: var(--text-primary);">
        ${escapeHtml(p.name)}
      </div>
      <div style="font-size: 0.78rem; color: var(--text-muted);">
        ${escapeHtml(p.category?.name || 'General')}
      </div>
    </div>
  </div>
</td>
          <td><code>${escapeHtml(p.sku)}</code></td>
          <td style="font-family: var(--font-heading); font-size: 1.1rem; font-weight: 700; color: var(--text-primary);">
            ${p.stock}
          </td>
          <td style="color: var(--text-secondary);">${threshold}</td>
          <td>${statusBadge}</td>
          <td>
            <button class="btn-action btn-success" onclick="openRestockForProduct(${p.id})">
              + Restock Units
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--danger); padding: 2rem;">Failed to load inventory: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function openRestockModal() {
  populateRestockProductsDropdown();
  openAdminModal('restock-modal');
}

function openRestockForProduct(productId) {
  populateRestockProductsDropdown(productId);
  openAdminModal('restock-modal');
}

function populateRestockProductsDropdown(selectedId = null) {
  const select = document.getElementById('restock-product-id');
  if (!select) return;

  select.innerHTML = adminState.products.map((p) => `
    <option value="${p.id}" ${selectedId === p.id ? 'selected' : ''}>
      #${p.id} — ${escapeHtml(p.name)} (Current: ${p.stock})
    </option>
  `).join('');
}

async function handleRestockSubmit(e) {
  e.preventDefault();
  const productId = parseInt(document.getElementById('restock-product-id').value, 10);
  const quantity = parseInt(document.getElementById('restock-quantity').value, 10);
  const reason = document.getElementById('restock-reason').value.trim();

  try {
    const res = await adminApiFetch('/api/inventory/restock', {
      method: 'POST',
      body: JSON.stringify({
        product_id: productId,
        quantity: quantity,
        reason: reason || 'Warehouse inventory replenishment',
      }),
    });

    showAdminToast(`Restocked ${quantity} units! New Stock: ${res.data.stock}`, 'success');
    closeAdminModal('restock-modal');

    // Refresh view
    loadInventoryTable();
    loadOverviewKPIs();
  } catch (err) {
    showAdminToast(err.message, 'error');
  }
}

// ============================================================================
// 4. Orders & State Machine
// ============================================================================
async function loadOrdersTable() {
  const tbody = document.getElementById('admin-orders-tbody');
  if (!tbody) return;

  try {
    const res = await adminApiFetch('/api/admin/orders');
    adminState.orders = res.data || [];
    renderOrdersRows();
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--danger); padding: 2rem;">Failed to load orders: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function filterAdminOrders() {
  renderOrdersRows();
}

function renderOrdersRows() {
  const tbody = document.getElementById('admin-orders-tbody');
  if (!tbody) return;

  const filterStatus = document.getElementById('admin-order-status-filter')?.value || '';
  const filtered = filterStatus
    ? adminState.orders.filter((o) => o.status === filterStatus)
    : adminState.orders;

  if (filtered.length === 0) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted); padding: 2rem;">No orders match the selected filter.</td></tr>`;
    return;
  }

  tbody.innerHTML = filtered.map((o) => {
    const statusClass = {
      PENDING: 'status-pending',
      CONFIRMED: 'status-processing',
      PROCESSING: 'status-processing',
      SHIPPED: 'status-shipped',
      DELIVERED: 'status-delivered',
      CANCELLED: 'status-cancelled',
    }[o.status] || 'status-pending';

    const isTerminal = o.status === 'DELIVERED' || o.status === 'CANCELLED';

    return `
      <tr>
        <td style="font-weight: 600; color: var(--text-primary);">#${o.id}</td>
        <td>User #${o.user_id}</td>
        <td style="font-size: 0.85rem; color: var(--text-muted);">${formatDate(o.created_at)}</td>
        <td>${o.items?.length || 0} line items</td>
        <td style="font-weight: 700; color: var(--text-primary);">$${(o.total_amount || 0).toFixed(2)}</td>
        <td><span class="status-badge ${statusClass}">${o.status}</span></td>
        <td>
          ${isTerminal ? `
            <span style="font-size: 0.8rem; color: var(--text-muted);">Terminal State</span>
          ` : `
            <button class="btn-action" onclick="openOrderStatusModal(${o.id}, '${o.status}')">
              Transition State →
            </button>
          `}
        </td>
      </tr>
    `;
  }).join('');
}

function openOrderStatusModal(orderId, currentStatus) {
  document.getElementById('modal-order-id').value = orderId;
  document.getElementById('modal-order-ref').textContent = `#${orderId}`;
  document.getElementById('modal-order-curr-status').textContent = currentStatus;

  // Compute allowed transitions
  const validTransitions = {
    PENDING: ['CONFIRMED', 'CANCELLED'],
    CONFIRMED: ['PROCESSING', 'CANCELLED'],
    PROCESSING: ['SHIPPED'],
    SHIPPED: ['DELIVERED'],
  }[currentStatus] || [];

  const select = document.getElementById('modal-order-next-status');
  select.innerHTML = validTransitions.map((st) => `
    <option value="${st}">${st}</option>
  `).join('');

  openAdminModal('order-status-modal');
}

async function handleOrderStatusSubmit(e) {
  e.preventDefault();
  const orderId = document.getElementById('modal-order-id').value;
  const nextStatus = document.getElementById('modal-order-next-status').value;

  try {
    await adminApiFetch(`/api/admin/orders/${orderId}/status`, {
      method: 'PUT',
      body: JSON.stringify({ status: nextStatus }),
    });

    showAdminToast(`Order #${orderId} transitioned to ${nextStatus}`, 'success');
    closeAdminModal('order-status-modal');

    loadOrdersTable();
    loadOverviewKPIs();
  } catch (err) {
    showAdminToast(err.message, 'error');
  }
}

// ============================================================================
// 5. Background Worker Pool Telemetry
// ============================================================================
async function loadWorkerStats() {
  try {
    const res = await adminApiFetch('/api/admin/workers/stats');
    const stats = res.data || {};
    adminState.workerStats = stats;

    document.getElementById('worker-pool-status').textContent = stats.status || 'ONLINE';
    const depth = stats.queue_depth ?? 0;
    const cap = stats.queue_capacity ?? 100;
    document.getElementById('worker-queue-ratio').textContent = `${depth} / ${cap}`;

    const percent = Math.min(100, Math.round((depth / cap) * 100));
    const bar = document.getElementById('worker-queue-bar');
    if (bar) bar.style.width = `${percent}%`;

    document.getElementById('worker-total-processed').textContent = (stats.total_processed_tasks ?? 0).toLocaleString();
    document.getElementById('worker-total-failed').textContent = (stats.total_failed_tasks ?? 0).toLocaleString();

    // Populate worker goroutines table
    const tbody = document.getElementById('admin-workers-tbody');
    if (tbody && stats.workers) {
      tbody.innerHTML = stats.workers.map((w) => `
        <tr>
          <td>
            <div style="font-weight: 600; color: var(--text-primary); font-family: monospace;">
              worker-goroutine-${w.id}
            </div>
          </td>
          <td>
            <span style="font-weight: 600; color: ${w.status === 'PROCESSING' ? 'var(--accent-secondary)' : 'var(--success)'};">
              ${w.status}
            </span>
          </td>
          <td style="font-weight: 600;">${w.processed_tasks || 0}</td>
          <td style="color: ${w.failed_tasks > 0 ? 'var(--danger)' : 'var(--text-muted)'};">${w.failed_tasks || 0}</td>
          <td>
            <span class="pulse-dot" style="${w.status === 'PROCESSING' ? 'background: var(--accent-secondary); box-shadow: 0 0 8px var(--accent-secondary);' : ''}"></span>
            <span style="font-size: 0.8rem; margin-left: 0.4rem; color: var(--text-secondary);">
              ${w.status === 'PROCESSING' ? 'Processing Payload' : 'Awaiting Channel Jobs'}
            </span>
          </td>
        </tr>
      `).join('');
    }
  } catch (err) {
    console.error('Failed to load worker stats:', err);
  }
}

// ============================================================================
// Categories Loading & Dropdowns
// ============================================================================
async function loadCategories() {
  try {
    const res = await adminApiFetch('/api/categories');
    adminState.categories = res.data || [];

    // Filter dropdown in products view
    const filterSelect = document.getElementById('admin-product-cat-filter');
    if (filterSelect) {
      filterSelect.innerHTML = `<option value="">All Categories</option>` +
        adminState.categories.map((c) => `<option value="${c.id}">${escapeHtml(c.name)}</option>`).join('');
    }
  } catch (err) {
    console.warn('Failed to load categories:', err);
  }
}

function populateCategoriesDropdown(selectId, selectedId = null) {
  const select = document.getElementById(selectId);
  if (!select) return;

  select.innerHTML = adminState.categories.map((c) => `
    <option value="${c.id}" ${selectedId === c.id ? 'selected' : ''}>
      ${escapeHtml(c.name)}
    </option>
  `).join('');
}

// ============================================================================
// Utilities
// ============================================================================
function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function formatDate(isoStr) {
  if (!isoStr) return '--';
  const d = new Date(isoStr);
  return d.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// ============================================================================
// Application Boot
// ============================================================================
async function bootDashboard() {
  await loadCategories();
  loadOverviewKPIs();

  // Background worker telemetry polling loop (every 3s)
  if (adminState.workerPollInterval) {
    clearInterval(adminState.workerPollInterval);
  }
  adminState.workerPollInterval = setInterval(() => {
    if (adminState.currentTab === 'workers' || adminState.currentTab === 'overview') {
      loadWorkerStats();
    }
  }, 3000);
}

window.addEventListener('DOMContentLoaded', async () => {
  const isAuthed = await verifyAdminAuth();
  if (isAuthed) {
    bootDashboard();
  }

  // Admin login form listener
  const loginForm = document.getElementById('admin-login-form');
  if (loginForm) {
    loginForm.addEventListener('submit', (e) => {
      e.preventDefault();
      const email = document.getElementById('gate-email').value.trim();
      const password = document.getElementById('gate-password').value;
      handleAdminLogin(email, password);
    });
  }

  // Product form listener
  const prodForm = document.getElementById('product-form');
  if (prodForm) prodForm.addEventListener('submit', saveProductForm);

  // Restock form listener
  const restockForm = document.getElementById('restock-form');
  if (restockForm) restockForm.addEventListener('submit', handleRestockSubmit);

  // Order status form listener
  const statusForm = document.getElementById('order-status-form');
  if (statusForm) statusForm.addEventListener('submit', handleOrderStatusSubmit);

  // Search input in products tab
  const prodSearch = document.getElementById('admin-product-search');
  if (prodSearch) {
    let timeout = null;
    prodSearch.addEventListener('input', () => {
      clearTimeout(timeout);
      timeout = setTimeout(loadProductsTable, 300);
    });
  }

  // Category filter in products tab
  const prodCatFilter = document.getElementById('admin-product-cat-filter');
  if (prodCatFilter) {
    prodCatFilter.addEventListener('change', loadProductsTable);
  }
});
