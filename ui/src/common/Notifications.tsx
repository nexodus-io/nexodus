import React, { useState, useCallback } from "react";
import Snackbar from "@mui/material/Snackbar";
import IconButton from "@mui/material/IconButton";
import CloseIcon from "@mui/icons-material/Close";

interface NotificationsProps {
  message: string | null;
  type: "success" | "error" | "info" | null;
}

const Notifications: React.FC<NotificationsProps> = ({ message, type }) => {
  if (!message || !type) return null;
  const [open, setOpen] = useState<boolean>(!!message);
  const handleClose = useCallback(() => {
    setOpen(false);
  }, []);

  return (
    <Snackbar
      open={open}
      autoHideDuration={6000}
      onClose={handleClose}
      message={message}
      action={
        <IconButton size="small" color="inherit" onClick={handleClose}>
          <CloseIcon fontSize="small" />
        </IconButton>
      }
      // Adjust styles based on the type
      style={{
        backgroundColor:
          type === "success" ? "green" : type === "error" ? "red" : "blue",
      }}
    />
  );
};

export default Notifications;
